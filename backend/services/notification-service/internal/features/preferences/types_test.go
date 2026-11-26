package preferences

import (
	"testing"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

func TestSecurityInboxCannotBeDisabled(t *testing.T) {
	for _, event := range []string{contract.LoginSucceeded, contract.PasswordChanged} {
		actual, err := (Channels{}).Effective(event)
		if err != nil || !actual.InAppEnabled || actual.EmailEnabled {
			t.Fatal("security inbox must survive opt-out; email must remain disabled")
		}
	}
	actual, err := (Channels{}).Effective(contract.EmailVerified)
	if err != nil || actual != (Channels{}) {
		t.Fatal("ordinary notifications must respect both channel preferences")
	}
	if _, err := (Channels{}).Effective("untrusted.event"); err == nil {
		t.Fatal("unknown event accepted")
	}
}
