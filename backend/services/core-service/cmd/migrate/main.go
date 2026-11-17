// migrate applies the core-service database migrations with the owner login.
//
//	go run ./cmd/migrate
//
// It reads CORE_MIGRATION_DATABASE_URL from .env.migrate (git-ignored); the API
// never loads that file or the owner login.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/database"
	"github.com/joho/godotenv"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "core database migration failed; check configuration, privileges and database availability")
		os.Exit(1)
	}

	fmt.Println("core database migrated")
}

func run() error {
	// Process environment wins over the file, like the API's .env handling.
	if _, err := os.Stat(".env.migrate"); err == nil {
		if godotenv.Load(".env.migrate") != nil {
			return database.ErrDatabase
		}
	} else if !os.IsNotExist(err) {
		return database.ErrDatabase
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, os.Getenv("CORE_MIGRATION_DATABASE_URL"))
	if err != nil {
		return err
	}

	defer pool.Close()

	return database.Migrate(ctx, pool)
}
