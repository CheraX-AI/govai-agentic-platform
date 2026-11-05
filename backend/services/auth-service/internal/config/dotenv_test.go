package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := LoadLocalEnv(path); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOVAI_DOTENV_EXISTING", "process")
	t.Setenv("GOVAI_DOTENV_NEW", "cleanup")
	if err := os.Unsetenv("GOVAI_DOTENV_NEW"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("GOVAI_DOTENV_EXISTING=file\nGOVAI_DOTENV_NEW=loaded\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := LoadLocalEnv(path); err != nil {
		t.Fatal(err)
	}

	if os.Getenv("GOVAI_DOTENV_EXISTING") != "process" || os.Getenv("GOVAI_DOTENV_NEW") != "loaded" {
		t.Fatal("incorrect environment precedence")
	}

	if err := os.WriteFile(path, []byte("PRIVATE=\"private\n"), 0600); err != nil {
		t.Fatal(err)
	}

	err := LoadLocalEnv(path)
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("unsafe or missing error: %v", err)
	}
}
