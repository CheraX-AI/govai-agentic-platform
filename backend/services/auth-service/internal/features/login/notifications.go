package login

import "context"

// LoginRecorder persists native-login success without waiting for RabbitMQ.

type LoginRecorder interface {
	RecordLogin(context.Context, string, string) error
}

// Browser login uses the ordinary constructor and records at session commit instead.

func NewServiceWithNotifications(
	provider PasswordProvider, verifier AccessVerifier, profiles ProfileLookup,
	recorder LoginRecorder, resolvers ...IdentifierResolver,
) (*Service, error) {
	if recorder == nil {
		return nil, ErrUnavailable
	}
	service, err := NewService(provider, verifier, profiles, resolvers...)
	if err != nil {
		return nil, err
	}
	service.notifications = recorder
	return service, nil
}
