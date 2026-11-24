package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/delivery"
	"github.com/joho/godotenv"
)

const sandboxHost = "sandbox.smtp.mailtrap.io"

type submitFunc func(context.Context, delivery.Config, delivery.Message) (delivery.Result, error)

// main sends only when explicitly requested against the Mailtrap sandbox host.

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, submit))
}

func run(args []string, output io.Writer, send submitFunc) int {
	flags := flag.NewFlagSet("smtp-check", flag.ContinueOnError)
	flags.SetOutput(output)
	sendEnabled := flags.Bool("send", false, "submit one synthetic message to the sandbox")
	envFile := flags.String("env", "", "optional local environment file")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if *envFile != "" {
		if godotenv.Load(*envFile) != nil {
			fmt.Fprintln(output, "Could not load local SMTP settings.")
			return 2
		}
	}
	settings, enabled, err := config.LoadSMTP()
	if err != nil || !enabled || !sandboxSettings(settings) {
		fmt.Fprintln(output, "Sandbox settings are incomplete or invalid; no message sent.")
		return 2
	}
	if !*sendEnabled {
		fmt.Fprintln(output, "Sandbox settings validated. No message sent; use -send explicitly.")
		return 0
	}
	identifier, err := newID()
	if err != nil {
		fmt.Fprintln(output, "Could not create a test message identifier.")
		return 1
	}
	message := delivery.Message{
		ID:        identifier,
		Recipient: "synthetic-citizen@govai.test",
		Subject:   "GovAI SMTP sandbox check",
		Body:      "Synthetic GovAI adapter test only.\nNo real citizen event or personal data.\n",
	}
	ctx, cancel := context.WithTimeout(context.Background(), settings.Timeout)
	defer cancel()
	result, err := send(ctx, settings, message)
	fmt.Fprintf(output, "SMTP status=%s code=%s retryable=%t\n",
		result.Status, result.Code, result.Retryable)
	if err != nil {
		if result.Status == "unknown" {
			fmt.Fprintln(output, "Do not resend automatically; inspect the sandbox first.")
		}
		return 1
	}
	fmt.Fprintln(output, "Test submission accepted. Check the Mailtrap sandbox inbox.")
	fmt.Fprintln(output, "Synthetic message ID:", identifier)
	return 0
}

func sandboxSettings(settings delivery.Config) bool {
	return settings.Host == sandboxHost &&
		(settings.TLSMode == "starttls" || settings.TLSMode == "tls") &&
		settings.Username != "" && settings.Password != "" &&
		settings.Username != "replace_with_sandbox_username" &&
		settings.Password != "replace_with_sandbox_password"
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		value[:4], value[4:6], value[6:8], value[8:10], value[10:]), nil
}

func submit(ctx context.Context, settings delivery.Config, message delivery.Message) (
	delivery.Result, error,
) {
	sender, err := delivery.New(settings)
	if err != nil {
		return delivery.Result{Status: "failed", Code: "invalid_configuration"}, err
	}
	return sender.Send(ctx, message)
}
