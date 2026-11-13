package permissionlab

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func TestCombinedFailureOutputRedacted(t *testing.T) {
	var out bytes.Buffer
	err := guardedCombined(
		func() error { panic("private-secret personal-data https://action.test/?key=private-token") },
		&out,
	)
	if err != errSafe || out.String() != "COMBINED_INTERNAL BLOCKED\n" {
		t.Fatal("private failure output escaped")
	}

	if strings.Contains(out.String(), "private") {
		t.Fatal("sensitive output")
	}
}

func TestCombinedSyntheticReservationContract(t *testing.T) {
	d := newCombinedStores()
	nic, err := registration.ParseNIC("000000101v")
	if err != nil {
		t.Fatal("fixture failed")
	}

	r := registration.Reservation{
		Issuer:      "https://issuer.test/realms/test",
		Key:         combinedKey(1),
		OperationID: "operation-one",
		Pending:     registration.ProfileData{FullName: "Synthetic", NIC: nic, MobileNumber: "+14155552671"},
	}
	s, err := d.Reserve(context.Background(), r)
	if err != nil {
		t.Fatal("synthetic reserve failed")
	}

	op := s.Snapshot()
	op.Revision++
	op.Phase = registration.SubjectRecorded
	op.Subject = "synthetic-subject"
	if err = s.Save(context.Background(), op); err != nil {
		t.Fatal("synthetic checkpoint failed")
	}

	if err = s.Close(context.Background()); err != nil {
		t.Fatal("synthetic release failed")
	}

	replay := r
	replay.OperationID = "must-not-replace"
	s, err = d.Reserve(context.Background(), replay)
	if err != nil || s.Snapshot().OperationID != r.OperationID {
		t.Fatal("synthetic replay overwrote ownership")
	}

	_ = s.Close(context.Background())
	conflict := r
	conflict.Key = combinedKey(2)
	conflict.OperationID = "other-operation"
	_, err = d.Reserve(context.Background(), conflict)
	if !errors.Is(err, registration.ErrNICConflict) {
		t.Fatal("synthetic NIC collision missed")
	}

	profile := registration.ProfileRecord{Issuer: r.Issuer, Subject: op.Subject, OperationID: r.OperationID, Data: r.Pending}
	if err = d.Persist(context.Background(), profile); err != nil {
		t.Fatal("synthetic profile bind failed")
	}

	profile.OperationID = "unrelated-owner"
	if err = d.Persist(context.Background(), profile); !errors.Is(err, registration.ErrProfileConflict) {
		t.Fatal("synthetic profile owner overwritten")
	}
}
