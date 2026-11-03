package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportComparisonScopeOrderAndChanges(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "order_only", true: "changed_permission"}[changed], func(t *testing.T) {
			dir := t.TempDir()
			for _, folder := range []string{"export", "rehearsal-export"} {
				if err := os.Mkdir(filepath.Join(dir, folder), 0700); err != nil {
					t.Fatal(err)
				}
				scopes := `["view","manage"]`
				if folder == "rehearsal-export" {
					scopes = `["manage","view"]`
					if changed {
						scopes = `["manage","extra"]`
					}
				}
				for _, realm := range []string{"GovAI", "master"} {
					data := object{"id": realm, "realm": realm, "clients": []any{object{"id": "client-id", "clientId": "synthetic", "authorizationSettings": object{"policies": []any{object{"name": "synthetic-permission", "type": "scope", "config": object{"scopes": scopes}}}}}}}
					b, err := json.Marshal(data)
					if err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(filepath.Join(dir, folder, realm+"-realm.json"), b, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			failed := false
			func() {
				defer func() {
					if recover() != nil {
						failed = true
					}
				}()
				compare(dir, "rehearsal-export")
			}()
			if failed != changed {
				t.Fatalf("comparison failed=%t changed=%t", failed, changed)
			}
		})
	}
}
