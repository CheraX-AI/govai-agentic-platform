// db explicitly checks or migrates the configured authentication database.
package main

import (
	"context"
	"fmt"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"os"
	"time"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "authentication database command failed; check configuration, privileges and database availability")
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "check") {
		return fmt.Errorf("usage: go run ./cmd/db migrate|check")
	}
	if os.Args[1] == "migrate" && config.LoadLocalEnv(".env.database-admin") != nil {
		return postgres.ErrStore
	}
	if config.LoadLocalEnv(".env.database") != nil || config.LoadLocalEnv(".env") != nil {
		return postgres.ErrStore
	}
	dsn := os.Getenv("DATABASE_URL")
	if os.Args[1] == "migrate" {
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
	} else {
		err = postgres.Ready(ctx, p)
	}
	if err != nil {
		return err
	}
	fmt.Println("authentication database " + os.Args[1] + " passed")
	return nil
}
