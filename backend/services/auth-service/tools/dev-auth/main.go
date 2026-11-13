// dev-auth only addresses the fixed local GovAI realm over validated TLS.
//
// Usage (from the auth-service folder):
//
//	go run ./tools/dev-auth certs [extra-host ...]   e.g. certs govai.example.ac.lk 10.0.0.5
//	go run ./tools/dev-auth <mode>                   e.g. configure, check, sessions-check
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/tools/internal/devsetup"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "DEV_AUTH FAILED (no secrets or remote bodies printed)")
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "certs" && len(os.Args) != 2) {
		return devsetup.ErrSetup
	}

	root, e := filepath.Abs(filepath.Join("..", "..", "..", "infrastructure", "keycloak"))
	if e != nil {
		return devsetup.ErrSetup
	}
	// Require invocation from the actual service directory, never a guessed cwd.
	if _, e = os.Stat("internal/features/registration/service.go"); e != nil {
		return devsetup.ErrSetup
	}

	if os.Args[1] == "certs" {
		fingerprint, e := devsetup.Certificates(filepath.Join(root, "tls"), os.Args[2:]...)
		if e != nil {
			return e
		}

		fmt.Println("DEV_AUTH_CERTS PASS; CA SHA256=" + fingerprint)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return devsetup.Run(ctx, root, os.Args[1], os.Stdout)
}
