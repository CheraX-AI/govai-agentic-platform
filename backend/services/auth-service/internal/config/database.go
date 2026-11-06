package config

import (
	"encoding/base64"
	"errors"
	"os"
)

// Database configuration is kept separate so ordinary health-only development
// remains possible. DSNs and keys must never be formatted into logs.
type Database struct {
	URL             string
	DataKey, NICKey []byte
}

func LoadDatabase() (Database, error) {
	var c Database
	c.URL = os.Getenv("DATABASE_URL")
	if c.URL == "" {
		return c, nil
	}
	var err error
	c.DataKey, err = base64.StdEncoding.DecodeString(os.Getenv("PROFILE_DATA_KEY"))
	if err != nil || len(c.DataKey) != 32 {
		return Database{}, errors.New("PROFILE_DATA_KEY must be a base64-encoded 32-byte key")
	}
	c.NICKey, err = base64.StdEncoding.DecodeString(os.Getenv("NIC_INDEX_KEY"))
	if err != nil || len(c.NICKey) != 32 {
		return Database{}, errors.New("NIC_INDEX_KEY must be a base64-encoded 32-byte key")
	}
	return c, nil
}
