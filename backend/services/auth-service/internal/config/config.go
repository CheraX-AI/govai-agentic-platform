// Package config reads and validates server settings.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/oidccheck"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	CORSOrigins      []string
	OIDCIssuerURL    string
	OIDCCheckTimeout time.Duration
}

// LoadLocalEnv loads only the explicitly named file; existing process settings win.
// Missing files are optional. Do not expose parser errors, which may contain secrets.
func LoadLocalEnv(path string) error {
	if err := godotenv.Load(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot load local environment file")
	}
	return nil
}

func Load() (Config, error) {
	port, exists := os.LookupEnv("PORT")
	if !exists {
		port = "5000"
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer between 1 and 65535")
	}
	origins := "http://localhost:5173,http://127.0.0.1:5173"
	if value, ok := os.LookupEnv("CORS_ALLOWED_ORIGINS"); ok {
		origins = value
	}
	var allowed []string
	if strings.TrimSpace(origins) != "" {
		for _, origin := range strings.Split(origins, ",") {
			origin = strings.TrimSpace(origin)
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(origin, "*\\ \t\r\n") {
				return Config{}, fmt.Errorf("CORS_ALLOWED_ORIGINS must contain exact HTTP(S) origins without paths or wildcards")
			}
			allowed = append(allowed, origin)
		}
	}
	issuer := os.Getenv("OIDC_ISSUER_URL")
	if issuer != "" && oidccheck.ValidateURL(issuer) != nil {
		return Config{}, fmt.Errorf("OIDC_ISSUER_URL must be an absolute HTTPS URL without credentials, query or fragment (HTTP allowed only on loopback)")
	}
	timeout := 5 * time.Second
	if value, ok := os.LookupEnv("OIDC_CHECK_TIMEOUT"); ok {
		var err error
		timeout, err = time.ParseDuration(value)
		if err != nil || timeout <= 0 || timeout > 30*time.Second {
			return Config{}, fmt.Errorf("OIDC_CHECK_TIMEOUT must be a positive duration at most 30s")
		}
	}
	return Config{Port: strconv.Itoa(number), CORSOrigins: allowed, OIDCIssuerURL: issuer, OIDCCheckTimeout: timeout}, nil
}
