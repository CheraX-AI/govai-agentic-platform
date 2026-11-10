package permissionlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

const registrationGroup = "ga48-registration"

func groupCreatePayload(username, password, operation string) object {
	return object{
		"username":        username,
		"email":           username + "@example.test",
		"enabled":         false,
		"emailVerified":   false,
		"requiredActions": []string{"VERIFY_EMAIL"},
		"attributes":      object{"govai_registration_operation": []string{operation}},
		"credentials":     []object{{"type": "password", "value": password, "temporary": false}},
		"groups":          []string{"/" + registrationGroup},
	}
}

func (l *lab) group(ctx context.Context, name string) string {
	p := "/admin/realms/" + l.realm
	l.must(ctx, "POST", p+"/groups", object{"name": name}, 201)
	groups := decode[[]object](l.must(ctx, "GET", p+"/groups?search="+name+"&exact=true", nil, 200))
	if len(groups) != 1 {
		panic(errSafe)
	}

	return groups[0]["id"].(string)
}

func (l *lab) groupCreate(ctx context.Context, token string) (string, string, int) {
	name, op := "group"+randomID(), randomID()
	p := "/admin/realms/" + l.realm
	status, _, err := l.request(ctx, token, "POST", p+"/users", groupCreatePayload(name, randomID(), op))
	if err != nil {
		panic(errSafe)
	}

	rows := decode[[]object](l.must(ctx, "GET", p+"/users?username="+name+"&exact=true", nil, 200))
	if status == 201 && len(rows) == 1 {
		return rows[0]["id"].(string), op, status
	}

	if status == 403 && len(rows) == 0 {
		return "", op, status
	}
	// A rejection with a remaining account or any unclear status is quarantined.
	fmt.Fprintln(l.out, "GROUP_CREATE_PROTOCOL BLOCKED")
	panic(errSafe)
}

func (l *lab) groupExperiment(ctx context.Context) error {
	for _, r := range []string{l.realm, l.other} {
		l.must(
			ctx,
			"POST",
			"/admin/realms",
			object{
				"realm":                     r,
				"enabled":                   true,
				"sslRequired":               "all",
				"registrationAllowed":       false,
				"verifyEmail":               true,
				"passwordPolicy":            "length(8)",
				"adminPermissionsEnabled":   true,
				"adminEventsEnabled":        false,
				"adminEventsDetailsEnabled": false,
			},
			201,
		)
	}

	p := "/admin/realms/" + l.realm
	profile := decode[object](l.must(ctx, "GET", p+"/users/profile", nil, 200))
	attributes := profile["attributes"].([]any)
	for _, v := range attributes {
		a := v.(map[string]any)
		if a["name"] == "firstName" || a["name"] == "lastName" {
			delete(a, "required")
		}
	}

	profile["attributes"] = append(
		attributes,
		object{
			"name":        "govai_registration_operation",
			"multivalued": false,
			"permissions": object{"view": []string{"admin"}, "edit": []string{"admin"}},
		},
	)
	l.must(ctx, "PUT", p+"/users/profile", profile, 200)
	l.must(ctx, "POST", p+"/roles", object{"name": "citizen"}, 201)
	citizen := decode[object](l.must(ctx, "GET", p+"/roles/citizen", nil, 200))
	realm := decode[object](l.must(ctx, "GET", p, nil, 200))
	defaultRole := realm["defaultRole"].(map[string]any)["name"].(string)
	l.must(ctx, "POST", p+"/roles/"+defaultRole+"/composites", []object{citizen}, 204)
	// Fixed synthetic group, not caller-selected and not a default group.
	group, control := l.group(ctx, registrationGroup), l.group(ctx, "ga48-control")
	_, secret, principal := l.createClient(ctx, "lab-group")
	permissions := l.clientID(ctx, "admin-permissions")
	policyBase := p + "/clients/" + permissions + "/authz/resource-server"
	policy := decode[object](
		l.must(
			ctx,
			"POST",
			policyBase+"/policy/user",
			object{
				"name":  "lab-group-principal",
				"type":  "user",
				"logic": "POSITIVE",
				"users": []string{principal},
			},
			201,
		),
	)
	// The four-scope exploratory run showed explicit group view was redundant.
	// This reduced candidate is tested end-to-end, not inferred from ablation alone.
	scopes := []string{"view-members", "manage-members", "manage-membership"}
	permission := decode[object](
		l.must(
			ctx,
			"POST",
			policyBase+"/permission/scope",
			groupPermission(scopes, group, policy["id"].(string)),
			201,
		),
	)
	fmt.Fprintln(l.out, "GROUP_SETUP PASS")
	// Observer proves this actor has no realm-management role mappings at all.
	rm := l.clientID(ctx, "realm-management")
	roles := decode[[]object](
		l.must(ctx, "GET", p+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite", nil, 200),
	)
	if len(roles) != 0 {
		panic(errSafe)
	}

	l.result("GROUP_NO_BROAD_ADMIN_ROLES", "PASS")
	token, err := l.token(ctx, l.realm, "lab-group", secret)
	if err != nil {
		return err
	}

	subject, op, status := l.groupCreate(ctx, token)
	if status != 201 {
		l.result("GROUP_CREATE", "FAIL_REQUIRED")
		return nil
	}

	l.verifyGroupIdentity(ctx, subject, op, group)
	l.result("GROUP_CREATE", "PASS")
	// Preserve this experiment's original group-bearing creation probe. The
	// production adapter now uses the same server-configured group representation;
	// read/search/enable/email methods are exercised as-is here.
	cfg := keycloak.RegistrationConfig{
		RegistrationGroupPath: "/" + registrationGroup,
		BaseURL:               l.base,
		Realm:                 l.realm,
		ClientID:              "lab-group",
		ClientSecret:          secret,
		VerificationClientID:  "lab-verification",
		VerificationRedirect:  "https://localhost/ga48-verified",
		VerificationLifespan:  time.Minute,
		CallTimeout:           5 * time.Second,
		MaxResponseBytes:      64 << 10,
		TokenSkew:             5 * time.Second,
	}
	a, err := keycloak.NewRegistrationAdmin(cfg, l.client)
	if err != nil {
		return err
	}

	defer a.Close()
	if _, err = a.ReadOwned(ctx, subject, op); err != nil {
		panic(errSafe)
	}

	l.result("GROUP_WITHOUT_QUERY_USERS_READ", "PASS")
	_, noQueryErr := a.FindOwned(ctx, op)
	if e, ok := noQueryErr.(*registration.AdminError); ok && e.Code == registration.AdminPermission {
		l.result("GROUP_WITHOUT_QUERY_USERS_RECONCILE", "FAIL_REQUIRED")
	} else if noQueryErr == nil {
		l.result("GROUP_WITHOUT_QUERY_USERS_RECONCILE", "PASS")
	} else {
		panic(errSafe)
	}
	// query-users opens the collection endpoint; per-user group visibility must
	// still be enforced. It grants neither view-users nor manage-users.
	l.grant(ctx, principal, rm, "query-users")
	roles = decode[[]object](
		l.must(ctx, "GET", p+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite", nil, 200),
	)
	if len(roles) != 1 || roles[0]["name"] != "query-users" {
		panic(errSafe)
	}

	a.Close()
	a, err = keycloak.NewRegistrationAdmin(cfg, l.client)
	if err != nil {
		return err
	}

	defer a.Close()
	token, err = l.token(ctx, l.realm, "lab-group", secret)
	if err != nil {
		return err
	}

	for _, membership := range []struct {
		id     string
		groups []string
	}{{"NONE", nil}, {"UNPERMITTED", []string{"/ga48-control"}}} {
		name := "outside" + randomID()
		body := groupCreatePayload(name, randomID(), randomID())
		if membership.groups == nil {
			delete(body, "groups")
		} else {
			body["groups"] = membership.groups
		}

		l.groupMutation(
			ctx,
			"GROUP_CREATE_"+membership.id,
			token,
			"POST",
			p+"/users",
			body,
			p+"/users?username="+name+"&exact=true",
		)
	}

	found, err := a.FindOwned(ctx, op)
	if err != nil || found.Subject != subject {
		panic(errSafe)
	}

	l.result("GROUP_READ_RECONCILE", "PASS")
	if err = a.EnableOwned(ctx, subject, op); err != nil {
		panic(errSafe)
	}

	state := decode[object](l.must(ctx, "GET", p+"/users/"+subject, nil, 200))
	if state["enabled"] != true || state["emailVerified"] != false {
		panic(errSafe)
	}

	l.result("GROUP_ENABLE", "PASS")
	if err = l.groupEmail(ctx, a, subject, op); err != nil {
		return err
	}

	for _, target := range []struct {
		id    string
		admin bool
	}{{"UNRELATED", false}, {"ADMIN", true}} {
		name := strings.ToLower(target.id) + randomID()
		victim := l.user(ctx, name, target.admin)
		l.groupInvisible(ctx, "GROUP_"+target.id, token, victim, name)
		l.probeUpdate(ctx, "GROUP_"+target.id, token, victim)
		l.probePassword(ctx, "GROUP_"+target.id, token, victim)
		l.groupMutation(
			ctx,
			"GROUP_IMPORT_"+target.id,
			token,
			"PUT",
			p+"/users/"+victim+"/groups/"+group,
			nil,
			p+"/users/"+victim+"/groups",
		)
	}

	l.groupMutation(
		ctx,
		"GROUP_REMOVE_MEMBERSHIP",
		token,
		"DELETE",
		p+"/users/"+subject+"/groups/"+group,
		nil,
		p+"/users/"+subject+"/groups",
	)
	l.groupMutation(
		ctx,
		"GROUP_RENAME",
		token,
		"PUT",
		p+"/groups/"+group,
		object{"name": "escaped"},
		p+"/groups/"+group,
	)
	l.groupMutation(
		ctx,
		"GROUP_CREATE_CHILD",
		token,
		"POST",
		p+"/groups/"+group+"/children",
		object{"name": "escape-child"},
		p+"/groups?briefRepresentation=false",
	)
	l.groupMutation(
		ctx,
		"GROUP_MOVE_UNRELATED",
		token,
		"POST",
		p+"/groups/"+group+"/children",
		object{"id": control, "name": "ga48-control"},
		p+"/groups?briefRepresentation=false",
	)
	l.groupMutation(
		ctx,
		"GROUP_MOVE_PERMITTED",
		token,
		"POST",
		p+"/groups/"+control+"/children",
		object{"id": group, "name": registrationGroup},
		p+"/groups?briefRepresentation=false",
	)
	// Candidate cannot edit its own authorization policy to remove restrictions.
	l.groupMutation(
		ctx,
		"GROUP_REWRITE_PERMISSION",
		token,
		"PUT",
		policyBase+"/permission/scope/"+permission["id"].(string),
		object{
			"name":         "lab-group-permission",
			"resourceType": "Users",
			"scopes":       []string{"manage", "view"},
			"policies":     []string{policy["id"].(string)},
		},
		policyBase+"/permission/scope/"+permission["id"].(string),
	)
	l.probeRoles(ctx, "GROUP", token, subject, rm)
	adminRole := decode[object](l.must(ctx, "GET", p+"/clients/"+rm+"/roles/realm-admin", nil, 200))
	l.groupMutation(
		ctx,
		"GROUP_ROLE_INHERITANCE",
		token,
		"POST",
		p+"/groups/"+group+"/role-mappings/clients/"+rm,
		[]object{adminRole},
		p+"/groups/"+group+"/role-mappings/clients/"+rm,
	)
	status, _, err = l.request(ctx, token, "GET", "/admin/realms/"+l.other+"/users", nil)
	if err != nil {
		panic(errSafe)
	}

	if status == 403 {
		l.result("GROUP_CROSS_REALM_READ", "PASS")
	} else {
		l.result("GROUP_CROSS_REALM_READ", "FAIL_UNSAFE")
	}

	cross := "cross" + randomID()
	l.groupMutation(
		ctx,
		"GROUP_CROSS_REALM_CREATE",
		token,
		"POST",
		"/admin/realms/"+l.other+"/users",
		object{"username": cross},
		"/admin/realms/"+l.other+"/users?username="+cross+"&exact=true",
	)
	// A different operation-owned user in the same group: app marker checks are
	// deliberately bypassed using direct Admin API calls, as a stolen actor could.
	other, otherOp, status := l.groupCreate(ctx, token)
	if status != 201 || otherOp == op {
		panic(errSafe)
	}

	l.verifyGroupIdentity(ctx, other, otherOp, group)
	if err = a.EnableOwned(ctx, other, otherOp); err != nil {
		panic(errSafe)
	}

	_, err = a.ReadOwned(ctx, other, op)
	var ae *registration.AdminError
	if !errorsAsOwnership(err, &ae) {
		panic(errSafe)
	}

	l.result("GROUP_MARKER_CHECK_IN_GO", "PASS")
	status, _, err = l.request(ctx, token, "GET", p+"/users/"+other, nil)
	if err != nil {
		panic(errSafe)
	}

	switch status {
	case 200:
		l.result("GROUP_OTHER_OPERATION_READ", "FAIL_UNSAFE")
	case 403:
		l.result("GROUP_OTHER_OPERATION_READ", "PASS")
	default:
		panic(errSafe)
	}

	l.probeUpdate(ctx, "GROUP_OTHER_OPERATION", token, other)
	l.probePassword(ctx, "GROUP_OTHER_OPERATION", token, other)
	// One-scope-at-a-time ablation uses fresh synthetic users. No retries of create.
	for _, missing := range scopes {
		reduced := []string{}
		for _, s := range scopes {
			if s != missing {
				reduced = append(reduced, s)
			}
		}

		body := groupPermission(reduced, group, policy["id"].(string))
		body["id"] = permission["id"]
		l.must(ctx, "PUT", policyBase+"/permission/scope/"+permission["id"].(string), body, 201)
		id := "GROUP_WITHOUT_" + strings.ToUpper(strings.ReplaceAll(missing, "-", "_"))
		u, operation, createStatus := l.groupCreate(ctx, token)
		if createStatus == 403 {
			l.result(id+"_CREATE", "FAIL_REQUIRED")
			continue
		}

		l.result(id+"_CREATE", "PASS")
		_, readErr := a.ReadOwned(ctx, u, operation)
		_, findErr := a.FindOwned(ctx, operation)
		if readErr != nil || findErr != nil {
			l.result(id+"_READ_RECONCILE", "FAIL_REQUIRED")
		} else {
			l.result(id+"_READ_RECONCILE", "PASS")
		}
	}
	// Restore the reviewed candidate only inside this disposable realm.
	body := groupPermission(scopes, group, policy["id"].(string))
	body["id"] = permission["id"]
	l.must(ctx, "PUT", policyBase+"/permission/scope/"+permission["id"].(string), body, 201)
	return nil
}

func (l *lab) groupInvisible(ctx context.Context, id, token, subject, name string) {
	p := "/admin/realms/" + l.realm
	status, _, err := l.request(ctx, token, "GET", p+"/users/"+subject, nil)
	if err != nil {
		panic(errSafe)
	}

	switch status {
	case 403:
		l.result(id+"_READ", "PASS")
	case 200:
		l.result(id+"_READ", "FAIL_UNSAFE")
	default:
		panic(errSafe)
	}

	status, data, err := l.request(ctx, token, "GET", p+"/users?username="+name+"&exact=true", nil)
	if err != nil || status != 200 {
		panic(errSafe)
	}

	if len(decode[[]object](data)) == 0 {
		l.result(id+"_SEARCH", "PASS")
	} else {
		l.result(id+"_SEARCH", "FAIL_UNSAFE")
	}
}

func groupPermission(scopes []string, group, policy string) object {
	return object{
		"name":             "lab-group-permission",
		"type":             "scope",
		"resourceType":     "Groups",
		"resources":        []string{group},
		"scopes":           scopes,
		"policies":         []string{policy},
		"decisionStrategy": "UNANIMOUS",
	}
}

func errorsAsOwnership(err error, target **registration.AdminError) bool {
	if e, ok := err.(*registration.AdminError); ok {
		*target = e
		return e.Code == registration.AdminOwnership
	}

	return false
}

func (l *lab) verifyGroupIdentity(ctx context.Context, subject, op, group string) {
	p := "/admin/realms/" + l.realm + "/users/" + subject
	u := decode[object](l.must(ctx, "GET", p, nil, 200))
	if u["enabled"] != false ||
		u["emailVerified"] != false ||
		!reflect.DeepEqual(u["requiredActions"], []any{"VERIFY_EMAIL"}) ||
		!reflect.DeepEqual(u["attributes"].(map[string]any)["govai_registration_operation"], []any{op}) {
		panic(errSafe)
	}

	groups := decode[[]object](l.must(ctx, "GET", p+"/groups", nil, 200))
	if len(groups) != 1 || groups[0]["id"] != group {
		panic(errSafe)
	}

	creds := decode[[]object](l.must(ctx, "GET", p+"/credentials", nil, 200))
	if len(creds) != 1 || creds[0]["type"] != "password" {
		panic(errSafe)
	}

	roles := decode[[]object](l.must(ctx, "GET", p+"/role-mappings/realm/composite", nil, 200))
	has := false
	for _, r := range roles {
		if r["name"] == "citizen" {
			has = true
		}
	}

	if !has {
		panic(errSafe)
	}

	l.result("GROUP_INITIAL_STATE_CITIZEN", "PASS")
}

// groupMutation requires an actual denial and independently unchanged state.
// Success, even if it happens to be a no-op, never becomes a passing denial.
func (l *lab) groupMutation(ctx context.Context, id, token, method, path string, body any, observe string) {
	before := decode[any](l.must(ctx, "GET", observe, nil, 200))
	status, _, err := l.request(ctx, token, method, path, body)
	if err != nil {
		fmt.Fprintln(l.out, id, "BLOCKED")
		panic(errSafe)
	}

	after := decode[any](l.must(ctx, "GET", observe, nil, 200))
	if status == 403 && reflect.DeepEqual(before, after) {
		l.result(id, "PASS")
	} else if status >= 200 && status < 300 {
		l.result(id, "FAIL_UNSAFE")
	} else {
		fmt.Fprintln(l.out, id, "BLOCKED")
		panic(errSafe)
	}
}

func (l *lab) groupEmail(ctx context.Context, a *keycloak.Admin, subject, op string) error {
	p := "/admin/realms/" + l.realm
	l.must(
		ctx,
		"POST",
		p+"/clients",
		object{
			"clientId":                  "lab-verification",
			"enabled":                   true,
			"protocol":                  "openid-connect",
			"publicClient":              true,
			"standardFlowEnabled":       true,
			"directAccessGrantsEnabled": false,
			"implicitFlowEnabled":       false,
			"redirectUris":              []string{"https://localhost/ga48-verified"},
		},
		201,
	)
	l.must(
		ctx,
		"PUT",
		p,
		object{
			"smtpServer": object{
				"host":     "lab-mail",
				"port":     "1025",
				"from":     "noreply@ga48.test",
				"auth":     "false",
				"ssl":      "false",
				"starttls": "false",
			},
		},
		204,
	)
	user := decode[object](l.must(ctx, "GET", p+"/users/"+subject, nil, 200))
	email := user["email"].(string)
	if err := a.SendVerification(ctx, subject, op); err != nil {
		return errSafe
	}

	mailClient := &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer mailClient.CloseIdleConnections()
	for i := 0; i < 15; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", l.mailBase+"/api/v1/messages", nil)
		if err != nil {
			return errSafe
		}

		response, err := mailClient.Do(req)
		if err != nil {
			return errSafe
		}

		data, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		response.Body.Close()
		if readErr != nil || len(data) > 1<<20 || response.StatusCode != 200 {
			return errSafe
		}

		var inbox struct {
			Messages []struct {
				To      []struct{ Address string }
				Subject string
			}
		}
		if json.Unmarshal(data, &inbox) != nil {
			return errSafe
		}

		for _, m := range inbox.Messages {
			for _, to := range m.To {
				if to.Address == email && strings.Contains(strings.ToLower(m.Subject), "verify") {
					l.result("GROUP_VERIFICATION_EMAIL", "PASS")
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return errSafe
		case <-time.After(time.Second):
		}
	}

	return errSafe
}
