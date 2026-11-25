package postgres

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"strings"
	"testing"
)

func TestPhotoReferencePersistence(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if MigrateCitizenDetails(ctx, admin) != nil {
		t.Fatal("migration")
	}
	owner := detailsCitizen(t, a, "123456789v", true)
	other := detailsCitizen(t, a, "223456789v", true)
	key := "citizen-photos/" + strings.Repeat("a", 64) + ".jpg"
	saved, err := a.SetCitizenPhoto(ctx, owner.Issuer, owner.Subject, 0, key)
	if err != nil || saved.Revision != 1 {
		t.Fatal("photo reference save failed", err)
	}
	loaded, err := b.CitizenDetails(ctx, owner.Issuer, owner.Subject)
	if err != nil || loaded.PhotoObjectKey != key {
		t.Fatal("reference not persisted")
	}
	if _, err = a.SetCitizenPhoto(ctx, owner.Issuer, owner.Subject, 0, key); !errors.Is(err, profile.ErrConflict) {
		t.Fatal("stale photo overwrite")
	}
	if _, err = a.SetCitizenPhoto(ctx, "https://wrong.test", owner.Subject, 0, key); !errors.Is(err, profile.ErrNotFound) {
		t.Fatal("issuer bypass")
	}
	if _, err = a.SetCitizenPhoto(ctx, owner.Issuer, owner.Subject, 1, "https://evil.test/object"); !errors.Is(err, profile.ErrInvalid) {
		t.Fatal("external key accepted")
	}
	untouched, err := b.CitizenDetails(ctx, other.Issuer, other.Subject)
	if err != nil || untouched.PhotoObjectKey != "" {
		t.Fatal("other citizen modified")
	}
}
