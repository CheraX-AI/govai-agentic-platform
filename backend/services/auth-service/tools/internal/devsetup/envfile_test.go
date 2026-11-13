package devsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestUpdateEnvReplacesInPlaceAndKeepsComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	original := "# -- Server\r\nPORT=5001\r\n\r\n# -- Login\r\n# keep me\r\nLOGIN_ENABLED=false\r\nLOGIN_CLIENT_ID=old\r\n"
	if e := os.WriteFile(path, []byte(original), 0600); e != nil {
		t.Fatal(e)
	}

	if e := updateEnv(path, "Login", map[string]string{"LOGIN_ENABLED": "true", "LOGIN_CLIENT_ID": "new"}); e != nil {
		t.Fatal(e)
	}

	b, _ := os.ReadFile(path)
	text := string(b)
	for _, want := range []string{"# -- Server\nPORT=5001\n", "# keep me\n", "LOGIN_ENABLED=\"true\"\n", "LOGIN_CLIENT_ID=\"new\"\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in updated file", want)
		}
	}

	if strings.Count(text, "LOGIN_ENABLED") != 1 || strings.Contains(text, "\r") {
		t.Fatal("key duplicated or line endings not normalised")
	}
}

func TestUpdateEnvAppendsNewKeysUnderSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if e := os.WriteFile(path, []byte("PORT=5001\n"), 0600); e != nil {
		t.Fatal(e)
	}

	if e := updateEnv(path, "Registration", map[string]string{"REGISTRATION_REALM": "GovAI", "REGISTRATION_ENABLED": "false"}); e != nil {
		t.Fatal(e)
	}

	b, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(b), "\n# -- Registration\nREGISTRATION_ENABLED=\"false\"\nREGISTRATION_REALM=\"GovAI\"\n") {
		t.Fatal("new keys not appended in sorted order under the section header")
	}

	env, e := godotenv.Read(path)
	if e != nil || env["PORT"] != "5001" || env["REGISTRATION_REALM"] != "GovAI" {
		t.Fatal("updated file does not parse back to the expected values")
	}
}

func TestUpdateEnvCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if e := updateEnv(path, "API authorization", map[string]string{"API_AUTH_ENABLED": "true"}); e != nil {
		t.Fatal(e)
	}

	env, e := godotenv.Read(path)
	if e != nil || env["API_AUTH_ENABLED"] != "true" {
		t.Fatal("missing file was not created")
	}
}

func TestUpdateEnvRejectsDuplicatedKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if e := os.WriteFile(path, []byte("PORT=5000\nPORT=5001\n"), 0600); e != nil {
		t.Fatal(e)
	}

	if updateEnv(path, "Server", map[string]string{"PORT": "5002"}) == nil {
		t.Fatal("duplicated key must be rejected")
	}

	b, _ := os.ReadFile(path)
	if string(b) != "PORT=5000\nPORT=5001\n" {
		t.Fatal("file changed despite the error")
	}
}
