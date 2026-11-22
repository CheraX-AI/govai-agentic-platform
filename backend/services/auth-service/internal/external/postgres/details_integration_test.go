package postgres

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func detailsCitizen(t *testing.T, store *Store, nic string, enabled bool) registration.ProfileRecord {
	t.Helper()
	random := id()
	key := random[:8] + "-" + random[8:12] + "-" + random[12:16] + "-" + random[16:20] + "-" + random[20:]
	r := testReservation(nic, key)
	session, err := store.Reserve(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSession(t, session)
	advance(t, session, registration.Creating, "")
	advance(t, session, registration.SubjectRecorded, id())
	o := session.Snapshot()
	record := registration.ProfileRecord{OperationID: o.OperationID, Issuer: o.Issuer, Subject: o.Subject, Data: o.Pending}
	if err := store.Persist(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if enabled {
		advance(t, session, registration.ProfileReady, "")
		advance(t, session, registration.Enabling, "")
		advance(t, session, registration.Enabled, "")
	}
	return record
}

func TestCitizenDetailsDatabase(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	before := detailsCitizen(t, a, "123456789v", true)
	var original []byte
	if admin.QueryRow(ctx, "SELECT profile_ciphertext FROM auth.citizen_profiles WHERE issuer=$1 AND subject=$2", before.Issuer, before.Subject).Scan(&original) != nil {
		t.Fatal("missing original registration")
	}
	if CitizenDetailsReady(ctx, a.pool) == nil {
		t.Fatal("missing migration reported ready")
	}
	if err := MigrateCitizenDetails(ctx, admin); err != nil {
		t.Fatal("migration failed", err)
	}
	if err := MigrateCitizenDetails(ctx, admin); err != nil {
		t.Fatal("migration replay failed", err)
	}
	if CitizenDetailsReady(ctx, a.pool) != nil || CitizenDetailsReady(ctx, admin) == nil {
		t.Fatal("readiness/privileges failed")
	}
	if MigrateCitizenDetails(ctx, a.pool) == nil {
		t.Fatal("runtime migrated schema")
	}
	initial, err := a.CitizenDetails(ctx, before.Issuer, before.Subject)
	if err != nil || initial.Revision != 0 || !initial.Fields.Address.Empty() {
		t.Fatal("existing citizen not backfilled")
	}
	after := detailsCitizen(t, a, "223456789v", true)
	if value, err := a.CitizenDetails(ctx, after.Issuer, after.Subject); err != nil || value.Revision != 0 {
		t.Fatal("new registration did not initialize details")
	}
	pending := detailsCitizen(t, a, "323456789v", false)
	for _, owner := range [][2]string{{before.Issuer, "missing"}, {"https://other.test", before.Subject}, {pending.Issuer, pending.Subject}, {"", before.Subject}} {
		if _, err := a.CitizenDetails(ctx, owner[0], owner[1]); !errors.Is(err, profile.ErrNotFound) {
			t.Fatal("missing/wrong issuer/non-ready owner read accepted")
		}
		if _, err := a.SaveCitizenDetails(ctx, owner[0], owner[1], 0, profile.EditableDetails{}); !errors.Is(err, profile.ErrNotFound) {
			t.Fatal("missing/wrong issuer/non-ready owner update accepted")
		}
	}
	fields := profile.EditableDetails{NameSI: "සිංහල නම", NameTA: "தமிழ் பெயர்", DateOfBirth: "2000-02-29", Gender: "synthetic", Address: profile.Address{Line1: "Private synthetic street", Line2: "Unit 4", City: "Synthetic city"}}
	saved, err := a.SaveCitizenDetails(ctx, before.Issuer, before.Subject, 0, fields)
	if err != nil || saved.Revision != 1 || saved.CreatedAt.IsZero() || saved.UpdatedAt.Before(saved.CreatedAt) {
		t.Fatal("save failed", err)
	}
	loaded, err := b.CitizenDetails(ctx, before.Issuer, before.Subject)
	if err != nil || loaded.Fields.NameSI != fields.NameSI || loaded.Fields.NameTA != fields.NameTA || loaded.Fields.DateOfBirth != fields.DateOfBirth || loaded.Fields.Address.Line1 != fields.Address.Line1 || loaded.Revision != 1 {
		t.Fatal("details did not persist across store connections")
	}
	other, err := b.CitizenDetails(ctx, after.Issuer, after.Subject)
	if err != nil || other.Fields.NameSI != "" || !other.Fields.Address.Empty() || other.Revision != 0 {
		t.Fatal("citizen A edit affected citizen B")
	}
	if _, err := a.SaveCitizenDetails(ctx, before.Issuer, before.Subject, 0, profile.EditableDetails{}); !errors.Is(err, profile.ErrConflict) {
		t.Fatal("stale update accepted")
	}
	if _, err := a.SaveCitizenDetails(ctx, before.Issuer, before.Subject, -1, fields); !errors.Is(err, profile.ErrInvalid) {
		t.Fatal("negative revision accepted")
	}
	if _, err := a.SaveCitizenDetails(ctx, before.Issuer, before.Subject, 1, profile.EditableDetails{DateOfBirth: "2001-02-29"}); !errors.Is(err, profile.ErrInvalid) {
		t.Fatal("invalid date accepted")
	}
	var ciphertext, registered []byte
	if admin.QueryRow(ctx, "SELECT details_ciphertext FROM auth.citizen_details WHERE issuer=$1 AND subject=$2", before.Issuer, before.Subject).Scan(&ciphertext) != nil {
		t.Fatal("missing encrypted details")
	}
	for _, value := range []string{fields.NameSI, fields.NameTA, fields.DateOfBirth, fields.Gender, fields.Address.Line1, fields.Address.Line2, fields.Address.City} {
		if bytes.Contains(ciphertext, []byte(value)) {
			t.Fatal("plaintext personal data in database")
		}
	}
	if admin.QueryRow(ctx, "SELECT profile_ciphertext FROM auth.citizen_profiles WHERE issuer=$1 AND subject=$2", before.Issuer, before.Subject).Scan(&registered) != nil || !bytes.Equal(registered, original) {
		t.Fatal("registered fields overwritten")
	}
	if err := a.Persist(ctx, before); err != nil {
		t.Fatal("registration replay broken by details migration", err)
	}

	t.Run("constraints-and-grants", func(t *testing.T) {
		for _, query := range []string{
			"UPDATE auth.citizen_details SET issuer='changed'",
			"UPDATE auth.citizen_details SET subject='changed'",
			"UPDATE auth.citizen_details SET created_at=clock_timestamp()",
			"UPDATE auth.citizen_details SET updated_at=clock_timestamp()",
			"UPDATE auth.citizen_details SET details_ciphertext=NULL",
			"UPDATE auth.citizen_details SET revision=revision+2",
			"UPDATE auth.citizen_details SET revision=revision+1,photo_object_key='https://storage.test/private'",
			"UPDATE auth.citizen_details SET revision=revision+1,photo_object_key='../private'",
			"UPDATE auth.citizen_details SET revision=revision+1,details_ciphertext=decode('00','hex')",
			"UPDATE auth.citizen_details SET revision=revision+1,details_ciphertext=decode(repeat('00',8193),'hex')",
			"INSERT INTO auth.citizen_details(issuer,subject) VALUES('missing','missing')",
			"DELETE FROM auth.citizen_details",
			"TRUNCATE auth.citizen_details",
			"UPDATE auth.citizen_profiles SET profile_ciphertext=decode('00','hex')",
		} {
			if _, err := a.pool.Exec(ctx, query); err == nil {
				t.Fatal("unsafe write accepted", query)
			}
		}
		if _, err := a.pool.Exec(ctx, "INSERT INTO auth.citizen_details(issuer,subject) VALUES($1,$2)", before.Issuer, before.Subject); err == nil {
			t.Fatal("duplicate details accepted")
		}
	})

	t.Run("concurrent-save", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for _, store := range []*Store{a, b} {
			wg.Add(1)
			go func(store *Store) {
				defer wg.Done()
				_, err := store.SaveCitizenDetails(ctx, after.Issuer, after.Subject, 0, fields)
				results <- err
			}(store)
		}
		wg.Wait()
		close(results)
		passed, conflicts := 0, 0
		for err := range results {
			if err == nil {
				passed++
			} else if errors.Is(err, profile.ErrConflict) {
				conflicts++
			} else {
				t.Fatal("unexpected concurrent error", err)
			}
		}
		if passed != 1 || conflicts != 1 {
			t.Fatal("concurrent saves lost revision protection")
		}
	})

	t.Run("preserve-photo-and-clear-details", func(t *testing.T) {
		// GA-254 will provide the server-owned object storage operation. Seed only
		// its reference here to prove ordinary profile edits cannot clear it.
		if _, err := admin.Exec(ctx, "UPDATE auth.citizen_details SET photo_object_key='citizens/synthetic/photo.png',revision=revision+1 WHERE issuer=$1 AND subject=$2", before.Issuer, before.Subject); err != nil {
			t.Fatal(err)
		}
		cleared, err := a.SaveCitizenDetails(ctx, before.Issuer, before.Subject, 2, profile.EditableDetails{})
		if err != nil || cleared.Revision != 3 || cleared.PhotoObjectKey != "citizens/synthetic/photo.png" || !cleared.Fields.Address.Empty() || cleared.Fields.NameSI != "" || cleared.Fields.DateOfBirth != "" {
			t.Fatal("clear/preserve photo failed", err)
		}
		var isNull bool
		if admin.QueryRow(ctx, "SELECT details_ciphertext IS NULL FROM auth.citizen_details WHERE issuer=$1 AND subject=$2", before.Issuer, before.Subject).Scan(&isNull) != nil || !isNull {
			t.Fatal("details clear did not persist")
		}
	})

	t.Run("checksum-protection", func(t *testing.T) {
		var original string
		if err := admin.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=7").Scan(&original); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, "UPDATE public.govai_auth_migrations SET checksum=$1 WHERE version=7", strings.Repeat("0", 64)); err != nil {
			t.Fatal(err)
		}
		if CitizenDetailsReady(ctx, a.pool) == nil || MigrateCitizenDetails(ctx, admin) == nil {
			t.Fatal("altered migration checksum accepted")
		}
		if _, err := admin.Exec(ctx, "UPDATE public.govai_auth_migrations SET checksum=$1 WHERE version=7", original); err != nil {
			t.Fatal(err)
		}
	})
}
