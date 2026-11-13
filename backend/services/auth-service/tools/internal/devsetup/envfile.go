package devsetup

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

// envKeyLine matches a dotenv assignment and captures its key.
var envKeyLine = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_.]*)\s*=`)

// serviceEnv is the auth service's single private settings file, given the
// infrastructure/keycloak directory that the dev-auth commands run from.
func serviceEnv(root string) string {
	return filepath.Join(root, "..", "..", "backend", "services", "auth-service", ".env")
}

// updateEnv sets keys in a dotenv file without rewriting the rest of it. A key
// that is already present is replaced on its own line, so comments, sections
// and other keys keep their place. New keys are added at the end under a
// "# -- section" header. The file is replaced atomically and stays private (0600).
func updateEnv(path, section string, values map[string]string) error {
	data, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return ErrSetup
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := []string{}
	if strings.TrimSpace(text) != "" {
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	}

	done := map[string]bool{}
	for i, line := range lines {
		m := envKeyLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		value, ok := values[m[1]]
		if !ok {
			continue
		}

		if done[m[1]] {
			return ErrSetup // A duplicated key would make the effective value unclear.
		}

		formatted, e := envLine(m[1], value)
		if e != nil {
			return ErrSetup
		}

		lines[i] = formatted
		done[m[1]] = true
	}

	added := []string{}
	for key := range values {
		if !done[key] {
			added = append(added, key)
		}
	}

	sort.Strings(added)
	if len(added) > 0 {
		lines = append(lines, "", "# -- "+section)
		for _, key := range added {
			formatted, e := envLine(key, values[key])
			if e != nil {
				return ErrSetup
			}

			lines = append(lines, formatted)
		}
	}

	return replaceFile(path, []byte(strings.Join(lines, "\n")+"\n"))
}

// envLine formats one assignment with the same quoting godotenv writes.
func envLine(key, value string) (string, error) {
	return godotenv.Marshal(map[string]string{key: value})
}

// replaceFile writes a sibling temporary file and renames it over the target,
// so an interrupted write never leaves a half-written settings file.
func replaceFile(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".env-update-*")
	if e != nil {
		return ErrSetup
	}

	tmp := f.Name()
	_, we := f.Write(b)
	ce := f.Close()
	if we != nil || ce != nil || os.Chmod(tmp, 0600) != nil || os.Rename(tmp, path) != nil {
		os.Remove(tmp)
		return ErrSetup
	}

	return nil
}
