package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
)

func TestProfileServicePostgres(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if err := MigrateCitizenDetails(ctx, admin); err != nil {
		t.Fatal(err)
	}
	one := detailsCitizen(t, a, "123456789v", true)
	two := detailsCitizen(t, a, "223456789v", true)
	service, err := profile.NewService(a, profile.CitizenFieldRules{})
	if err != nil {
		t.Fatal(err)
	}
	caller := profile.Caller{Issuer: one.Issuer, Subject: one.Subject, Email: "synthetic@example.test"}
	first, err := service.Read(ctx, caller)
	if err != nil || first.Registered.FullName != one.Data.FullName || first.Registered.NIC.RestrictedOriginal() != one.Data.NIC.RestrictedOriginal() || first.Registered.MobileNumber != one.Data.MobileNumber {
		t.Fatal("authoritative registration not returned")
	}
	request := profile.UpdateRequest{Revision: 0, Fields: profile.EditableDetails{NameSI: "සිංහල නම", NameTA: "தமிழ் பெயர்", DateOfBirth: "2012-12-10", Gender: "male", Address: profile.Address{Line1: "Synthetic road"}}}
	if _, err := service.Update(ctx, caller, request); err != nil {
		t.Fatal(err)
	}
	reopened, _ := profile.NewService(b, profile.CitizenFieldRules{})
	loaded, err := reopened.Read(ctx, caller)
	if err != nil || loaded.Details.Revision != 1 || loaded.Details.Fields.NameTA != request.Fields.NameTA || loaded.Details.Fields.DateOfBirth != request.Fields.DateOfBirth || loaded.Details.Fields.Gender != "male" || loaded.Registered.NIC.RestrictedOriginal() != first.Registered.NIC.RestrictedOriginal() {
		t.Fatal("profile not persisted")
	}
	if _, err := reopened.Update(ctx, caller, request); !errors.Is(err, profile.ErrConflict) {
		t.Fatal("stale API service update accepted")
	}
	other, err := reopened.Read(ctx, profile.Caller{Issuer: two.Issuer, Subject: two.Subject})
	if err != nil || other.Registered.NIC.RestrictedOriginal() != two.Data.NIC.RestrictedOriginal() || other.Details.Fields.NameTA != "" {
		t.Fatal("profile owners mixed")
	}
	if _, err := a.RegisteredProfile(ctx, "https://other.test", one.Subject); !errors.Is(err, profile.ErrNotFound) {
		t.Fatal("wrong issuer read accepted")
	}
}
