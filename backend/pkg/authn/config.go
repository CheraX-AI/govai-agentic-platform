package authn

import (
	"net/url"
	"strings"
	"time"
)

// Config says which Keycloak tokens a service accepts.
type Config struct {
	Issuer   string           // realm URL, e.g. https://keycloak.example/realms/GovAI
	Audience string           // API name the token must be issued for ("aud")
	Clients  []string         // apps allowed to request tokens ("azp")
	Now      func() time.Time // clock; tests can replace it, defaults to time.Now
}

// validate rejects settings that would make token checks unsafe.
func (c Config) validate() error {
	issuer, err := url.Parse(c.Issuer)
	if err != nil ||
		issuer.Scheme != "https" ||
		issuer.Host == "" ||
		issuer.User != nil ||
		issuer.RawQuery != "" ||
		issuer.Fragment != "" ||
		strings.HasSuffix(c.Issuer, "/") {
		return ErrConfig
	}

	if c.Audience == "" || len(c.Clients) == 0 {
		return ErrConfig
	}

	for _, client := range c.Clients {
		if client == "" {
			return ErrConfig
		}
	}

	return nil
}
