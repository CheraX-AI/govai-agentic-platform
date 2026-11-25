package recovery

import (
	"context"
	"testing"
)

type passwordNotificationFixture struct {
	*fixture
	confirmed, uncertain, cancelled int
	failNotification                bool
}

func (fixture *passwordNotificationFixture) PasswordChanged(context.Context, Token) error {
	fixture.confirmed++
	if fixture.failNotification {
		return ErrUnavailable
	}
	return nil
}

func (fixture *passwordNotificationFixture) PasswordUncertain(context.Context, Token) error {
	fixture.uncertain++
	return nil
}

func (fixture *passwordNotificationFixture) PasswordCancelled(context.Context, Token) error {
	fixture.cancelled++
	return nil
}

func TestPasswordChangeNotificationCheckpoint(t *testing.T) {
	for _, test := range []struct {
		name, fail                      string
		failNotification                bool
		confirmed, uncertain, cancelled int
		wantSuccess                     bool
	}{
		{name: "success", confirmed: 1, wantSuccess: true},
		{name: "unconfirmed reset", fail: "reset", uncertain: 1},
		{name: "later logout failure", fail: "logout", confirmed: 1},
		{name: "before reset failure", fail: "recipient", cancelled: 1},
		{name: "outbox failure", failNotification: true, confirmed: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, base := setupService(t)
			adapter := &passwordNotificationFixture{
				fixture: base, failNotification: test.failNotification,
			}
			service.tokens = adapter
			if _, err := service.Request(context.Background(), "000000001V"); err != nil {
				t.Fatal("recovery fixture setup failed")
			}
			if adapter.confirmed+adapter.uncertain+adapter.cancelled != 0 {
				t.Fatal("requesting a reset produced a password-change checkpoint")
			}
			base.events = nil
			base.fail = test.fail
			err := service.Reset(context.Background(), base.token.Reveal(), "synthetic-password")
			if (err == nil) != test.wantSuccess || adapter.confirmed != test.confirmed ||
				adapter.uncertain != test.uncertain || adapter.cancelled != test.cancelled {
				t.Fatal("password notification outcome was incorrect")
			}
			if test.failNotification {
				loggedOut := false
				for _, event := range base.events {
					loggedOut = loggedOut || event == "logout"
				}
				if !loggedOut {
					t.Fatal("notification failure bypassed provider logout cleanup")
				}
			}
			if service.Reset(context.Background(), base.token.Reveal(), "another-password") == nil ||
				adapter.confirmed != test.confirmed {
				t.Fatal("reset replay created another notification")
			}
		})
	}
}
