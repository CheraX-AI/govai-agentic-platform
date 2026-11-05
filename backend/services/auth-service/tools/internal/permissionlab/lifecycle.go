package permissionlab

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
)

func (l *lab) lifeCheck(id string, ok bool) {
	if ok {
		l.result(id, "PASS")
	} else {
		l.result(id, "FAIL")
	}
}

func (l *lab) lifecycle(ctx context.Context) error {
	fmt.Fprintln(l.out, "LIFE_GROUP_AUTHORITY ACCEPTED_PROTOTYPE_LIMITATION")
	p := "/admin/realms/" + l.realm
	l.must(
		ctx,
		"POST",
		"/admin/realms",
		object{
			"realm":                     l.realm,
			"enabled":                   true,
			"sslRequired":               "all",
			"registrationAllowed":       false,
			"duplicateEmailsAllowed":    false,
			"verifyEmail":               true,
			"passwordPolicy":            "length(8)",
			"adminPermissionsEnabled":   true,
			"adminEventsEnabled":        false,
			"adminEventsDetailsEnabled": false,
			"defaultLocale":             "en",
			"smtpServer": object{
				"host":     "lab-mail",
				"port":     "1025",
				"from":     "noreply@ga48.test",
				"auth":     "false",
				"ssl":      "false",
				"starttls": "false",
			},
		},
		201,
	)
	profile := decode[object](l.must(ctx, "GET", p+"/users/profile", nil, 200))
	attrs := profile["attributes"].([]any)
	for _, v := range attrs {
		a := v.(map[string]any)
		if a["name"] == "firstName" || a["name"] == "lastName" {
			delete(a, "required")
		}
	}

	profile["attributes"] = append(
		attrs,
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
	l.must(
		ctx,
		"POST",
		p+"/roles/"+realm["defaultRole"].(map[string]any)["name"].(string)+"/composites",
		[]object{citizen},
		204,
	)
	group := l.group(ctx, registrationGroup)
	_, secret, principal := l.createClient(ctx, "lab-lifecycle")
	rm := l.clientID(ctx, "realm-management")
	policyBase := p + "/clients/" + l.clientID(ctx, "admin-permissions") + "/authz/resource-server"
	policy := decode[object](
		l.must(
			ctx,
			"POST",
			policyBase+"/policy/user",
			object{
				"name":  "lifecycle-principal",
				"type":  "user",
				"logic": "POSITIVE",
				"users": []string{principal},
			},
			201,
		),
	)
	l.must(
		ctx,
		"POST",
		policyBase+"/permission/scope",
		groupPermission(
			[]string{"view-members", "manage-members", "manage-membership"},
			group,
			policy["id"].(string),
		),
		201,
	)
	roles := decode[[]object](
		l.must(ctx, "GET", p+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite", nil, 200),
	)
	l.lifeCheck("LIFE_NO_BROAD_ADMIN_ROLES", len(roles) == 0)
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
	if err := l.liveTLS(ctx); err != nil {
		return err
	}

	token, err := l.token(ctx, l.realm, "lab-lifecycle", secret)
	if err != nil {
		return err
	}
	// Short-password rejection: inspect authoritative state, not just HTTP status.
	shortName := "short" + randomID()
	status, _, err := l.request(ctx, token, "POST", p+"/users", groupCreatePayload(shortName, randomID()[:7], randomID()))
	if err != nil {
		return errSafe
	}

	rows := decode[[]object](l.must(ctx, "GET", p+"/users?username="+shortName+"&exact=true", nil, 200))
	fmt.Fprintln(l.out, "LIFE_PASSWORD_7_STATUS", fmt.Sprintf("HTTP_%d", status))
	l.lifeCheck("LIFE_PASSWORD_7_REJECTED", status == 400 && len(rows) == 0)
	// Exactly 8 ASCII characters, unchanged, satisfies this isolated length policy.
	name, op := "valid"+randomID(), randomID()
	subject, status := l.lifeCreate(ctx, token, name, randomID()[:8], op)
	if status != 201 {
		l.lifeCheck("LIFE_PASSWORD_8_ACCEPTED", false)
		return nil
	}

	l.lifeCheck("LIFE_PASSWORD_8_ACCEPTED", true)
	l.verifyGroupIdentity(ctx, subject, op, group)
	l.lifeCheck("LIFE_DISABLED_UNVERIFIED_CITIZEN", true)
	l.grant(ctx, principal, rm, "query-users")
	roles = decode[[]object](
		l.must(ctx, "GET", p+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite", nil, 200),
	)
	l.lifeCheck("LIFE_SCOPED_PRINCIPAL", len(roles) == 1 && roles[0]["name"] == "query-users")
	token, err = l.token(ctx, l.realm, "lab-lifecycle", secret)
	if err != nil {
		return err
	}

	userPath := p + "/users/" + subject
	before := decode[any](l.must(ctx, "GET", userPath, nil, 200))
	creds := decode[any](l.must(ctx, "GET", userPath+"/credentials", nil, 200))
	for _, kind := range []string{"USERNAME", "EMAIL"} {
		duplicate := groupCreatePayload(name, randomID(), randomID())
		if kind == "USERNAME" {
			duplicate["email"] = "different" + randomID() + "@example.test"
		} else {
			duplicate["username"] = "different" + randomID()
		}

		s, _, e := l.request(ctx, token, "POST", p+"/users", duplicate)
		if e != nil {
			return errSafe
		}

		after := decode[any](l.must(ctx, "GET", userPath, nil, 200))
		afterCreds := decode[any](l.must(ctx, "GET", userPath+"/credentials", nil, 200))
		matches := decode[[]object](l.must(ctx, "GET", p+"/users?email="+name+"%40example.test&exact=true", nil, 200))
		l.lifeCheck(
			"LIFE_DUPLICATE_"+kind,
			s == 409 &&
				len(matches) == 1 &&
				matches[0]["id"] == subject &&
				reflect.DeepEqual(before, after) &&
				reflect.DeepEqual(creds, afterCreds),
		)
	}

	cfg := keycloak.RegistrationConfig{
		RegistrationGroupPath: "/" + registrationGroup,
		BaseURL:               l.base,
		Realm:                 l.realm,
		ClientID:              "lab-lifecycle",
		ClientSecret:          secret,
		VerificationClientID:  "lab-verification",
		VerificationRedirect:  "https://localhost/ga48-verified",
		VerificationLifespan:  60 * time.Second,
		CallTimeout:           5 * time.Second,
		MaxResponseBytes:      64 << 10,
		TokenSkew:             5 * time.Second,
	}
	a, err := keycloak.NewRegistrationAdmin(cfg, l.client)
	if err != nil {
		return err
	}

	defer a.Close()
	found, err := a.FindOwned(ctx, op)
	if err != nil {
		return errSafe
	}

	l.lifeCheck("LIFE_RECONCILIATION", found.Subject == subject)
	if err = a.EnableOwned(ctx, subject, op); err != nil {
		return errSafe
	}

	u := decode[object](l.must(ctx, "GET", userPath, nil, 200))
	l.lifeCheck(
		"LIFE_ENABLE",
		u["enabled"] == true &&
			u["emailVerified"] == false &&
			reflect.DeepEqual(u["requiredActions"], []any{"VERIFY_EMAIL"}),
	)
	if err = a.SendVerification(ctx, subject, op); err != nil {
		return errSafe
	}

	link, err := l.verificationLink(ctx, name+"@example.test")
	if err != nil {
		return err
	}

	l.lifeCheck("LIFE_MAILPIT_DELIVERY", true)
	invalid, err := tamperLink(link)
	if err != nil {
		return err
	}

	invalidStatus, invalidPage, err := l.visitVerification(ctx, invalid)
	if err != nil {
		return err
	}

	u = decode[object](l.must(ctx, "GET", userPath, nil, 200))
	l.lifeCheck("LIFE_INVALID_LINK", u["emailVerified"] == false && invalidStatus >= 400)
	l.linkObservation("LIFE_INVALID_OUTCOME", invalidStatus, invalidPage)
	validStatus, validPage, err := l.visitVerification(ctx, link)
	if err != nil {
		return err
	}

	u = decode[object](l.must(ctx, "GET", userPath, nil, 200))
	l.lifeCheck(
		"LIFE_VERIFY_COMPLETION",
		u["emailVerified"] == true && !containsAction(u, "VERIFY_EMAIL"),
	)
	l.linkObservation("LIFE_VALID_OUTCOME", validStatus, validPage)
	reusedStatus, reusedPage, err := l.visitVerification(ctx, link)
	if err != nil {
		return err
	}

	unchanged := decode[object](l.must(ctx, "GET", userPath, nil, 200))
	l.lifeCheck("LIFE_REUSED_STATE_UNCHANGED", reflect.DeepEqual(u, unchanged))
	l.linkObservation("LIFE_REUSED_OUTCOME", reusedStatus, reusedPage)
	// Separate user and one-second action token: test expiry without altering clocks.
	expName, expOp := "expire"+randomID(), randomID()
	expSubject, expStatus := l.lifeCreate(ctx, token, expName, randomID(), expOp)
	if expStatus != 201 {
		return errSafe
	}

	if err = a.EnableOwned(ctx, expSubject, expOp); err != nil {
		return errSafe
	}

	cfg.VerificationLifespan = time.Second
	short, err := keycloak.NewRegistrationAdmin(cfg, l.client)
	if err != nil {
		return err
	}

	defer short.Close()
	if err = short.SendVerification(ctx, expSubject, expOp); err != nil {
		return errSafe
	}

	expLink, err := l.verificationLink(ctx, expName+"@example.test")
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return errSafe
	case <-time.After(3 * time.Second):
	}

	expHTTP, expPage, err := l.visitVerification(ctx, expLink)
	if err != nil {
		return err
	}

	u = decode[object](l.must(ctx, "GET", p+"/users/"+expSubject, nil, 200))
	l.lifeCheck("LIFE_EXPIRED_LINK", u["emailVerified"] == false && expHTTP >= 400)
	l.linkObservation("LIFE_EXPIRED_OUTCOME", expHTTP, expPage)
	return nil
}

func containsAction(u object, action string) bool {
	v, _ := u["requiredActions"].([]any)
	for _, a := range v {
		if a == action {
			return true
		}
	}

	return false
}

func (l *lab) lifeCreate(ctx context.Context, token, name, password, op string) (string, int) {
	p := "/admin/realms/" + l.realm
	status, _, err := l.request(ctx, token, "POST", p+"/users", groupCreatePayload(name, password, op))
	if err != nil {
		panic(errSafe)
	}

	rows := decode[[]object](l.must(ctx, "GET", p+"/users?username="+name+"&exact=true", nil, 200))
	if status == 201 && len(rows) == 1 {
		return rows[0]["id"].(string), status
	}

	if len(rows) != 0 {
		panic(errSafe)
	}

	return "", status
}

func (l *lab) liveTLS(ctx context.Context) error {
	transport := l.client.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.InsecureSkipVerify {
		return errSafe
	}

	for _, wrongHost := range []bool{false, true} {
		t := transport.Clone()
		endpoint := l.base
		if wrongHost {
			u, _ := url.Parse(l.base)
			target := u.Host
			t.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, target)
			}
			endpoint = "https://wrong-host.invalid:" + u.Port()
		} else {
			t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: x509.NewCertPool()}
		}

		c := &http.Client{Transport: t, Timeout: 3 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint+"/admin/realms/"+l.realm, nil)
		resp, err := c.Do(req)
		if resp != nil {
			resp.Body.Close()
		}

		c.CloseIdleConnections()
		var unknown x509.UnknownAuthorityError
		var hostname x509.HostnameError
		ok := errors.As(err, &unknown)
		id := "LIFE_TLS_UNTRUSTED"
		if wrongHost {
			ok = errors.As(err, &hostname)
			id = "LIFE_TLS_HOSTNAME"
		}

		l.lifeCheck(id, ok)
	}

	return nil
}

func (l *lab) mailJSON(ctx context.Context, path string, target any) error {
	c := &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer c.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, "GET", l.mailBase+path, nil)
	if err != nil {
		return errSafe
	}

	r, err := c.Do(req)
	if err != nil {
		return errSafe
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 || r.StatusCode != 200 || json.Unmarshal(body, target) != nil {
		return errSafe
	}

	return nil
}

var messageID = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

var httpsLink = regexp.MustCompile(`https://[^\s<>"']+`)

var hrefLink = regexp.MustCompile(`(?i)<a\b[^>]*\bhref=["']([^"']+)["']`)

func (l *lab) verificationLink(ctx context.Context, email string) (string, error) {
	for i := 0; i < 15; i++ {
		var inbox struct {
			Messages []struct {
				ID string
				To []struct{ Address string }
			}
		}
		if err := l.mailJSON(ctx, "/api/v1/messages", &inbox); err != nil {
			return "", err
		}

		for _, m := range inbox.Messages {
			for _, to := range m.To {
				if to.Address == email && messageID.MatchString(m.ID) {
					var msg struct{ Text, HTML string }
					if err := l.mailJSON(ctx, "/api/v1/message/"+m.ID, &msg); err != nil {
						return "", err
					}

					for _, raw := range httpsLink.FindAllString(html.UnescapeString(msg.Text+"\n"+msg.HTML), -1) {
						if link, ok := l.safeAction(raw); ok {
							return link, nil
						}
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return "", errSafe
		case <-time.After(time.Second):
		}
	}

	return "", errSafe
}

func (l *lab) safeAction(raw string) (string, bool) {
	u, err := url.Parse(html.UnescapeString(raw))
	base, _ := url.Parse(l.base)
	if err != nil ||
		u.Scheme != "https" ||
		u.Host != base.Host ||
		u.User != nil ||
		u.Fragment != "" ||
		u.Path != "/realms/"+l.realm+"/login-actions/action-token" ||
		u.Query().Get("key") == "" {
		return "", false
	}

	return u.String(), true
}

func tamperLink(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errSafe
	}

	q := u.Query()
	parts := strings.Split(q.Get("key"), ".")
	if len(parts) != 3 || parts[2] == "" {
		return "", errSafe
	}

	c := "A"
	if parts[2][0] == 'A' {
		c = "B"
	}

	parts[2] = c + parts[2][1:]
	q.Set("key", strings.Join(parts, "."))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// New cookie jar for each link test; only explicit same-origin action-token links
// are followed. No login forms, token exchanges or external redirects are visited.
func (l *lab) visitVerification(ctx context.Context, raw string) (int, string, error) {
	current, ok := l.safeAction(raw)
	if !ok {
		return 0, "", errSafe
	}

	jar, _ := cookiejar.New(nil)
	c := *l.client
	c.Jar = jar
	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", current, nil)
		if err != nil {
			return 0, "", errSafe
		}

		r, err := c.Do(req)
		if err != nil {
			return 0, "", errSafe
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		r.Body.Close()
		if err != nil || len(body) > 1<<20 {
			return 0, "", errSafe
		}

		page := classifyVerification(string(body))
		// Keycloak's generic stale-link error may mention already-completed or
		// expired actions. Do not misreport those examples as an observed success.
		if r.StatusCode >= 400 {
			page = "ERROR_PAGE"
		}

		next := ""
		for _, match := range hrefLink.FindAllStringSubmatch(string(body), -1) {
			if v, ok := l.safeAction(match[1]); ok && v != current {
				next = v
				break
			}
		}

		if next == "" {
			return r.StatusCode, page, nil
		}

		current = next
	}

	return 0, "", errSafe
}

func classifyVerification(body string) string {
	// Keycloak embeds translations for other outcomes in scripts. Those strings
	// are not the visible response and must not determine the observed result.
	v := strings.ToLower(
		html.UnescapeString(visibleTags.ReplaceAllString(nonVisible.ReplaceAllString(body, " "), " ")),
	)
	for _, p := range []struct{ needle, code string }{
		{"already been verified", "ALREADY_VERIFIED"},
		{"already verified", "ALREADY_VERIFIED"},
		{"has been verified already", "ALREADY_VERIFIED"},
		{"expired", "EXPIRED"},
		{"invalid", "INVALID"},
		{"successfully verified", "VERIFIED"},
		{"email address has been verified", "VERIFIED"},
		{"confirm", "CONFIRMATION"},
	} {
		if strings.Contains(v, p.needle) {
			return p.code
		}
	}

	return "OTHER"
}

var nonVisible = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>|<style\b[^>]*>.*?</style>`)

var visibleTags = regexp.MustCompile(`<[^>]+>`)

func (l *lab) linkObservation(id string, status int, page string) {
	fmt.Fprintln(l.out, id, fmt.Sprintf("HTTP_%d", status), page)
}
