package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var ErrConfig = errors.New("invalid notification configuration")

type Config struct {
	Address          string
	BrokerURL        string
	BrokerCA         string
	SecurityPrefetch int
	NormalPrefetch   int
}

// Load validates settings without including credentials in returned errors.

func Load() (Config, error) {
	address := value("NOTIFICATION_LISTEN_ADDRESS", "127.0.0.1:5003")
	host, port, err := net.SplitHostPort(address)
	if err != nil || !validPort(port) || host == "" {
		return Config{}, ErrConfig
	}

	brokerHost := value("RABBITMQ_HOST", "127.0.0.1")
	brokerPort := value("RABBITMQ_PORT", "5672")
	username := os.Getenv("RABBITMQ_CONSUMER_USER")
	password := os.Getenv("RABBITMQ_CONSUMER_PASSWORD")
	vhost := value("RABBITMQ_VHOST", "govai")
	if !validPort(brokerPort) || username == "" || password == "" ||
		strings.ContainsAny(brokerHost, "/\\?#@ \t\r\n") || brokerHost == "" ||
		vhost != "govai" {
		return Config{}, ErrConfig
	}

	scheme := value("RABBITMQ_SCHEME", "amqps")
	switch scheme {
	case "amqp":
		development := os.Getenv("NOTIFICATION_ENV") == "development"
		local := brokerHost == "127.0.0.1" || brokerHost == "localhost" ||
			brokerHost == "::1" || brokerHost == "rabbitmq"
		if !development || !local {
			return Config{}, ErrConfig
		}
	case "amqps":
	default:
		return Config{}, ErrConfig
	}

	security, err := prefetch("NOTIFICATION_SECURITY_PREFETCH", 10)
	if err != nil {
		return Config{}, err
	}
	normal, err := prefetch("NOTIFICATION_NORMAL_PREFETCH", 2)
	if err != nil {
		return Config{}, err
	}

	endpoint := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(brokerHost, brokerPort),
		Path:   "/" + vhost,
		User:   url.UserPassword(username, password),
	}
	return Config{
		Address:          address,
		BrokerURL:        endpoint.String(),
		BrokerCA:         os.Getenv("RABBITMQ_CA_FILE"),
		SecurityPrefetch: security,
		NormalPrefetch:   normal,
	}, nil
}

func value(name, fallback string) string {
	if setting := os.Getenv(name); setting != "" {
		return setting
	}
	return fallback
}

func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port > 0 && port <= 65535 &&
		strconv.Itoa(port) == value
}

func prefetch(name string, fallback int) (int, error) {
	setting := value(name, strconv.Itoa(fallback))
	limit, err := strconv.Atoi(setting)
	if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != setting {
		return 0, ErrConfig
	}
	return limit, nil
}
