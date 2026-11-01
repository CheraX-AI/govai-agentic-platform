package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var ErrConfig = errors.New("invalid core chat server configuration")

type Config struct {
	Port, Certificate, Key, Issuer, Audience, CAFile string
	Clients, Origins                                 []string
}

func Load() (Config, error) {
	c := Config{Port: os.Getenv("CORE_PORT"), Certificate: os.Getenv("CORE_TLS_CERT_FILE"), Key: os.Getenv("CORE_TLS_KEY_FILE"), Issuer: os.Getenv("CHAT_ISSUER_URL"), Audience: os.Getenv("CHAT_AUDIENCE"), CAFile: os.Getenv("CHAT_CA_FILE")}
	if c.Port == "" {
		c.Port = "5002"
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != c.Port || c.Certificate == "" || c.Key == "" || c.Audience == "" {
		return Config{}, ErrConfig
	}
	issuer, err := url.Parse(c.Issuer)
	if err != nil || issuer.Scheme != "https" || issuer.Hostname() == "" || issuer.User != nil || issuer.RawQuery != "" || issuer.ForceQuery || issuer.Fragment != "" || strings.HasSuffix(c.Issuer, "/") {
		return Config{}, ErrConfig
	}
	for _, value := range strings.Split(os.Getenv("CHAT_ALLOWED_CLIENTS"), ",") {
		v := strings.TrimSpace(value)
		if v == "" {
			return Config{}, ErrConfig
		}
		c.Clients = append(c.Clients, v)
	}
	if len(c.Clients) > 10 {
		return Config{}, ErrConfig
	}
	if origins := os.Getenv("CHAT_ALLOWED_ORIGINS"); origins != "" {
		for _, value := range strings.Split(origins, ",") {
			origin := strings.TrimSpace(value)
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.String() != origin {
				return Config{}, ErrConfig
			}
			c.Origins = append(c.Origins, origin)
		}
	}
	return c, nil
}
