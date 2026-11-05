package permissionlab

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGroupPayloadHasFixedMembershipAndNoRoleOverride(t *testing.T) {
	p := groupCreatePayload("synthetic", "transient", "operation")
	if len(p) != 8 ||
		p["enabled"] != false ||
		p["emailVerified"] != false ||
		!reflect.DeepEqual(p["groups"], []string{"/ga48-registration"}) {
		t.Fatal("unsafe create shape")
	}

	for _, key := range []string{"realmRoles", "clientRoles", "nic", "full_name", "mobile_number"} {
		if _, ok := p[key]; ok {
			t.Fatal("unexpected identity/profile override")
		}
	}

	if !reflect.DeepEqual(p["requiredActions"], []string{"VERIFY_EMAIL"}) {
		t.Fatal("verification action missing")
	}

	if p["credentials"].([]object)[0]["temporary"] != false {
		t.Fatal("credential changed")
	}
}

func TestGroupPermissionStaysResourceScoped(t *testing.T) {
	scopes := []string{"view-members", "manage-members", "manage-membership"}
	p := groupPermission(scopes, "group-id", "principal-policy")
	if p["resourceType"] != "Groups" ||
		!reflect.DeepEqual(p["resources"], []string{"group-id"}) ||
		!reflect.DeepEqual(p["scopes"], scopes) {
		t.Fatal("permission widened")
	}

	if !reflect.DeepEqual(p["policies"], []string{"principal-policy"}) {
		t.Fatal("principal binding missing")
	}
}

func TestGroupMutationUsesIndependentObserverAndStrictDenials(t *testing.T) {
	for _, tc := range []struct {
		status  int
		changed bool
		want    string
	}{{403, false, "PASS"}, {204, false, "FAIL_UNSAFE"}, {204, true, "FAIL_UNSAFE"}} {
		t.Run(tc.want+http.StatusText(tc.status), func(t *testing.T) {
			state := false
			reads := 0
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					if r.Header.Get("Authorization") != "Bearer independent-observer" {
						t.Error("actor used as oracle")
					}

					reads++
					_ = json.NewEncoder(w).Encode(object{"changed": state})
					return
				}

				if r.Header.Get("Authorization") != "Bearer actor" {
					t.Error("observer used for attack")
				}

				if tc.changed {
					state = true
				}

				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, "private upstream response")
			}))
			defer s.Close()
			var out bytes.Buffer
			l := &lab{
				base:     s.URL,
				realm:    testRealm,
				other:    otherRealm,
				client:   s.Client(),
				observer: "independent-observer",
				out:      &out,
			}
			l.groupMutation(
				context.Background(),
				"CASE",
				"actor",
				"PUT",
				"/admin/realms/"+testRealm+"/users/victim",
				object{"enabled": false},
				"/admin/realms/"+testRealm+"/users/victim",
			)
			if reads != 2 || out.String() != "CASE "+tc.want+"\n" {
				t.Fatal("denial assertion weakened")
			}
		})
	}
}

func TestGroupModePreservesExistingStateDirectory(t *testing.T) {
	var out bytes.Buffer
	if RunGroups(context.Background(), t.TempDir(), &out) != 1 || out.String() != "LAB_EXECUTION BLOCKED\n" {
		t.Fatal("group mode overwrote existing resources")
	}
}
