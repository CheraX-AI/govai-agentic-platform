package app

import "github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"

// Config holds the settings of every feature, loaded once at startup.
type Config struct {
	Base          config.Config
	Database      config.Database
	Registration  config.Registration
	Login         config.Login
	Authorization config.Authorization
	Web           config.Web
	WebChat       config.WebChat
	Recovery      config.Recovery
}

// LoadConfig reads envFile (process environment settings win) and validates
// every feature's settings. A missing envFile is allowed.
func LoadConfig(envFile string) (Config, error) {
	if err := config.LoadLocalEnv(envFile); err != nil {
		return Config{}, err
	}

	var c Config
	var err error
	if c.Base, err = config.Load(); err != nil {
		return Config{}, err
	}

	if c.Database, err = config.LoadDatabase(); err != nil {
		return Config{}, err
	}

	if c.Registration, err = config.LoadRegistration(); err != nil {
		return Config{}, err
	}

	if c.Login, err = config.LoadLogin(); err != nil {
		return Config{}, err
	}

	if c.Authorization, err = config.LoadAuthorization(); err != nil {
		return Config{}, err
	}

	if c.Web, err = config.LoadWeb(); err != nil {
		return Config{}, err
	}

	if c.Recovery, err = config.LoadRecovery(c.Registration, c.Web); err != nil {
		return Config{}, err
	}

	if c.WebChat, err = config.LoadWebChat(c.Web.Enabled); err != nil {
		return Config{}, err
	}

	// Optional rate-limit overrides, shared by login, web sessions and registration.
	limits, err := config.LoadRateLimits()
	if err != nil {
		return Config{}, err
	}

	c.Login.Limits = limits
	c.Web.Limits = limits
	c.Registration.Limits = limits
	return c, nil
}

// tlsEnabled reports whether any credential feature is on; those require HTTPS.
func (c Config) tlsEnabled() bool {
	return c.Registration.Enabled ||
		c.Login.Enabled ||
		c.Authorization.Enabled ||
		c.Web.Enabled ||
		c.Recovery.Enabled
}
