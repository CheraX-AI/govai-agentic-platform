// permission-lab is an explicitly opted-in, disposable Keycloak experiment.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/tools/internal/permissionlab"
)

func main() {
	optIn := flag.Bool(
		"allow-disposable-experiment",
		false,
		"create a separate Keycloak container/storage and probe synthetic accounts",
	)
	state := flag.String(
		"state-dir",
		"",
		"new private directory outside Git for certificates and a non-secret resource manifest",
	)
	experiment := flag.String(
		"experiment",
		"baseline",
		"baseline, groups or lifecycle; each uses fresh disposable resources",
	)
	flag.Parse()
	if !*optIn ||
		*state == "" ||
		(*experiment != "baseline" &&
			*experiment != "groups" &&
			*experiment != "lifecycle") {
		fmt.Println("LAB_OPT_IN BLOCKED")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var code int
	if *experiment == "lifecycle" {
		code = permissionlab.RunLifecycle(ctx, *state, os.Stdout)
	} else if *experiment == "groups" {
		code = permissionlab.RunGroups(ctx, *state, os.Stdout)
	} else {
		code = permissionlab.Run(ctx, *state, os.Stdout)
	}

	os.Exit(code)
}
