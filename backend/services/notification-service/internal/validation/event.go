package validation

import contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"

// DecodeEvent checks payload and broker routing before future persistence.

func DecodeEvent(body []byte, issuer, routingKey, queue string) (contract.Event, error) {
	event, err := contract.Decode(body, issuer)
	if err != nil {
		return contract.Event{}, err
	}
	priority, err := contract.PriorityFor(event.Type)
	if err != nil {
		return contract.Event{}, err
	}
	expectedQueue := "notifications.account.v1"
	retryKey := "delivery.retry.v1"
	if priority == contract.High {
		expectedQueue = "notifications.security.v1"
		retryKey = "delivery.security.retry.v1"
	}
	if queue != expectedQueue || (routingKey != event.Type && routingKey != retryKey) {
		return contract.Event{}, contract.ErrInvalid
	}
	return event, nil
}
