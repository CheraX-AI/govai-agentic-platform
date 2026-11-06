// Command oidc-check performs a separate, bounded, read-only provider check.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/oidccheck"
)

func run() error {
	if err := config.LoadLocalEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return oidccheck.Check(context.Background(), cfg.OIDCIssuerURL, cfg.OIDCCheckTimeout)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("OIDC connectivity check passed: discovery issuer matched and nonempty JWKS retrieved (not token validation)")
}
