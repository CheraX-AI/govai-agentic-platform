// web-maintenance explicitly inspects or reconciles one configured web client's
// recovery queue. It never loads owner credentials or starts a background worker.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/app/wiring"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/websession"
)

func main() {
	if run() != nil {
		fmt.Fprintln(
			os.Stderr,
			"web maintenance failed; check private configuration, database, provider and queue status",
		)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "status" && os.Args[1] != "reconcile") {
		return websession.ErrUnavailable
	}

	if config.LoadLocalEnv(".env") != nil {
		return websession.ErrUnavailable
	}

	c, err := config.LoadWeb()
	if err != nil || !c.Enabled {
		return websession.ErrUnavailable
	}

	database, err := config.LoadDatabase()
	if err != nil {
		return websession.ErrUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, database.URL)
	if err != nil {
		return websession.ErrUnavailable
	}

	defer pool.Close()
	if os.Args[1] == "status" {
		queue, err := wiring.WebRecoveryQueue(ctx, c, pool)
		if err != nil {
			return websession.ErrUnavailable
		}

		status, err := queue.RevocationStatus(ctx)
		if err != nil {
			return websession.ErrUnavailable
		}

		fmt.Printf(
			"web revocations: pending=%d running=%d completed=%d attention=%d\n",
			status.Pending,
			status.Running,
			status.Completed,
			status.Attention,
		)
		return nil
	}

	result, err := wiring.ReconcileWeb(ctx, c, pool)
	if err != nil {
		return websession.ErrUnavailable
	}

	fmt.Printf("web reconciliation: completed=%d deferred=%d\n", result.Completed, result.Deferred)
	return nil
}
