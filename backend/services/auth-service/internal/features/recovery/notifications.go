package recovery

import (
	"context"
	"time"
)

// Optional durable checkpoints track a claimed reset before its provider mutation.

type PasswordNotifications interface {
	PasswordChanged(context.Context, Token) error
	PasswordUncertain(context.Context, Token) error
	PasswordCancelled(context.Context, Token) error
}

func (service *Service) passwordNotification(ctx context.Context, token Token, state string) error {
	checkpoints, enabled := service.tokens.(PasswordNotifications)
	if !enabled {
		return nil
	}

	// A confirmed provider change still needs recording after request cancellation.

	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	switch state {
	case "confirmed":
		return checkpoints.PasswordChanged(cleanup, token)
	case "uncertain":
		return checkpoints.PasswordUncertain(cleanup, token)
	case "cancelled":
		return checkpoints.PasswordCancelled(cleanup, token)
	default:
		return ErrUnavailable
	}
}
