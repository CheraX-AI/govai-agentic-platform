package config

import (
	"encoding/base64"
	"testing"
)

func TestDatabaseKeys(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, e := LoadDatabase(); e != nil {
		t.Fatal(e)
	}

	t.Setenv("DATABASE_URL", "postgres://app@127.0.0.1/db?sslmode=disable")
	t.Setenv("PROFILE_DATA_KEY", "")
	if _, e := LoadDatabase(); e == nil {
		t.Fatal("missing key accepted")
	}

	t.Setenv("PROFILE_DATA_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("NIC_INDEX_KEY", "bad")
	if _, e := LoadDatabase(); e == nil {
		t.Fatal("invalid index key accepted")
	}

	t.Setenv("NIC_INDEX_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if _, e := LoadDatabase(); e != nil {
		t.Fatal(e)
	} // Separate-key enforcement is in NewStore.
}
