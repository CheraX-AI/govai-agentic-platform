// db explicitly checks or migrates the configured authentication database.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
)

func main() {
	if run() != nil {
		fmt.Fprintln(
			os.Stderr,
			"authentication database command failed; check configuration, privileges and database availability",
		)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 ||
		(os.Args[1] != "migrate" &&
			os.Args[1] != "check" &&
			os.Args[1] != "migrate-web" &&
			os.Args[1] != "check-web" &&
			os.Args[1] != "prune-web" &&
			os.Args[1] != "prune-web-jobs" &&
			os.Args[1] != "migrate-recovery" &&
			os.Args[1] != "check-recovery") {
		return fmt.Errorf(
			"usage: go run ./tools/db migrate|check|migrate-web|check-web|prune-web|prune-web-jobs|migrate-recovery|check-recovery",
		)
	}

	isMigration := os.Args[1] == "migrate" || os.Args[1] == "migrate-web" || os.Args[1] == "migrate-recovery"
	isPrivileged := isMigration || os.Args[1] == "prune-web" || os.Args[1] == "prune-web-jobs"
	// Only privileged commands load the owner login; the API never reads .env.migrate.
	if isPrivileged && config.LoadLocalEnv(".env.migrate") != nil {
		return postgres.ErrStore
	}

	if config.LoadLocalEnv(".env") != nil {
		return postgres.ErrStore
	}

	dsn := os.Getenv("DATABASE_URL")
	if isPrivileged {
		dsn = os.Getenv("MIGRATION_DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, err := postgres.Open(ctx, dsn)
	if err != nil {
		return err
	}

	defer p.Close()
	if os.Args[1] == "migrate" {
		err = postgres.Migrate(ctx, p)
	} else if os.Args[1] == "migrate-web" {
		err = postgres.MigrateWebSessions(ctx, p)
	} else if os.Args[1] == "migrate-recovery" {
		err = postgres.MigrateRecoverySessions(ctx, p)
	} else if os.Args[1] == "check-recovery" {
		err = postgres.RecoveryReady(ctx, p)
	} else if os.Args[1] == "check-web" {
		err = postgres.WebSessionsReady(ctx, p)
	} else if os.Args[1] == "prune-web" {
		var count int64
		count, err = postgres.PruneWebSessions(ctx, p, 24*time.Hour, 500)
		if err == nil {
			fmt.Printf("expired web session records removed: %d\n", count)
		}
	} else if os.Args[1] == "prune-web-jobs" {
		var count int64
		count, err = postgres.PruneCompletedWebRevocations(ctx, p, 24*time.Hour, 500)
		if err == nil {
			fmt.Printf("completed web revocation records removed: %d\n", count)
		}
	} else {
		err = postgres.Ready(ctx, p)
	}

	if err != nil {
		return err
	}

	fmt.Println("authentication database " + os.Args[1] + " passed")
	return nil
}
