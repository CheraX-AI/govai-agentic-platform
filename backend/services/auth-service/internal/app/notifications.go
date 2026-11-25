package app

import (
	"context"
	"log/slog"
	"sync"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/app/wiring"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/rabbitmq"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5/pgxpool"
)

type emailVerificationWorker interface {
	Run(context.Context) error
}

type notificationWorker interface {
	Run(context.Context, contract.Priority) error
}

// Only the central flag can enable hooks; require one issuer across auth features.

func (cfg Config) prepareNotificationEvents() (Config, error) {
	cfg.Login.NotificationIssuer = ""
	cfg.Web.NotificationIssuer = ""
	cfg.Recovery.NotificationIssuer = ""
	if !cfg.NotificationEvents.Enabled {
		return cfg, nil
	}
	issuer := cfg.NotificationEvents.Publisher.Issuer
	if cfg.Database.URL == "" {
		return Config{}, notificationevents.ErrConfiguration
	}
	if _, err := rabbitmq.New(cfg.NotificationEvents.Publisher); err != nil {
		return Config{}, notificationevents.ErrConfiguration
	}
	if cfg.Login.Enabled && cfg.Login.Provider.Issuer != issuer ||
		cfg.Web.Enabled && cfg.Web.Provider.Issuer != issuer ||
		cfg.Recovery.Enabled && cfg.Recovery.Service.Issuer != issuer ||
		cfg.Registration.Enabled &&
			cfg.Registration.Admin.BaseURL+"/realms/"+cfg.Registration.Admin.Realm != issuer {
		return Config{}, notificationevents.ErrConfiguration
	}
	cfg.Login.NotificationIssuer = issuer
	cfg.Web.NotificationIssuer = issuer
	cfg.Recovery.NotificationIssuer = issuer
	return cfg, nil
}

// Schema readiness is required; a live broker connection is not a login prerequisite.

func (app *App) prepareNotificationWorker(
	ctx context.Context, cfg Config, pool *pgxpool.Pool,
) error {
	if !cfg.NotificationEvents.Enabled {
		return nil
	}
	if postgres.NotificationOutboxReady(ctx, pool) != nil {
		return notificationevents.ErrConfiguration
	}
	store, err := postgres.NewNotificationDispatchStore(pool,
		cfg.NotificationEvents.Publisher.Issuer)
	if err != nil {
		return notificationevents.ErrConfiguration
	}
	publisher, err := rabbitmq.New(cfg.NotificationEvents.Publisher)
	if err != nil {
		return notificationevents.ErrConfiguration
	}
	app.notifications, err = notificationevents.New(store, publisher)
	if err != nil {
		return err
	}
	if cfg.Registration.Enabled {
		observer, closeObserver, err := wiring.NotificationEmailObserver(
			ctx, cfg.Registration, pool, cfg.NotificationEvents.Publisher.Issuer,
		)
		if err != nil {
			return err
		}
		app.emailVerifications = observer
		app.onClose(closeObserver)
	}
	return nil
}

// Run owns these goroutines and joins them before returning to pool cleanup.

func (app *App) startNotificationWorkers(ctx context.Context) func() {
	if app.notifications == nil && app.emailVerifications == nil {
		return func() {}
	}
	workerContext, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	if app.notifications != nil {
		for _, lane := range []contract.Priority{contract.High, contract.Normal} {
			workers.Add(1)
			go func(priority contract.Priority) {
				defer workers.Done()
				err := app.notifications.Run(workerContext, priority)
				if err != nil && workerContext.Err() == nil {
					slog.Error("authentication notification worker stopped")
				}
			}(lane)
		}
	}
	if app.emailVerifications != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			err := app.emailVerifications.Run(workerContext)
			if err != nil && workerContext.Err() == nil {
				slog.Error("email verification observer stopped")
			}
		}()
	}
	var once sync.Once
	return func() {
		once.Do(func() { cancel(); workers.Wait() })
	}
}
