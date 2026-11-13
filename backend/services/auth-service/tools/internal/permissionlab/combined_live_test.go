package permissionlab

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

// TestCombinedLive is the sole live entry point. Ordinary test runs skip it.
// All synthetic persistence types are test-only, not runtime fallback stores.
func TestCombinedLive(t *testing.T) {
	if os.Getenv("GA48_COMBINED_LIVE") != "1" {
		t.Skip("COMBINED_LIVE opt-in disabled")
	}

	state := os.Getenv("GA48_COMBINED_STATE_DIR")
	if state == "" {
		t.Fatal("COMBINED_STATE BLOCKED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	if err := safeCombinedRun(ctx, state, os.Stdout); err != nil {
		t.Fatal("COMBINED_LIVE FAIL_OR_BLOCKED")
	}

	if logs.Len() != 0 {
		t.Fatal("COMBINED_LOGS FAIL")
	}

	fmt.Println("COMBINED_LOGS PASS")
}

func safeCombinedRun(ctx context.Context, state string, out io.Writer) (err error) {
	return guardedCombined(func() error {
		return runWithScenario(
			ctx,
			state,
			out,
			"combined",
			func(ctx context.Context, l *lab) error { return l.combinedScenario(ctx) },
		)
	}, out)
}

func guardedCombined(action func() error, out io.Writer) (err error) {
	defer func() {
		if recover() != nil {
			fmt.Fprintln(out, "COMBINED_INTERNAL BLOCKED")
			err = errSafe
		}
	}()
	return action()
}

// Calls always reach the REAL adapter. One explicit fault discards its successful
// create result after Keycloak committed, modelling a lost result at this boundary
// rather than claiming to reproduce an actual TCP timeout.
type combinedAdmin struct {
	registration.Admin
	stores                          *combinedStores
	issuer, key                     string
	creates, finds, enables, emails int
	loseCreateResult                bool
}

func (*combinedAdmin) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func (a *combinedAdmin) CreateDisabled(ctx context.Context, r registration.Request, op string) (registration.Identity, error) {
	if a.stores.operation(a.issuer, a.key).Phase != registration.Creating {
		panic(errSafe)
	}

	a.creates++
	u, err := a.Admin.CreateDisabled(ctx, r, op)
	if err == nil && a.loseCreateResult {
		a.loseCreateResult = false
		return registration.Identity{}, &registration.AdminError{Code: registration.AdminAmbiguous, UnknownOutcome: true}
	}

	return u, err
}

func (a *combinedAdmin) FindOwned(ctx context.Context, op string) (registration.Identity, error) {
	a.finds++
	return a.Admin.FindOwned(ctx, op)
}

func (a *combinedAdmin) EnableOwned(ctx context.Context, subject, op string) error {
	if !a.stores.hasProfile(a.issuer, subject, op) ||
		a.stores.operation(a.issuer, a.key).Phase != registration.Enabling {
		panic(errSafe)
	}

	a.enables++
	return a.Admin.EnableOwned(ctx, subject, op)
}

func (a *combinedAdmin) SendVerification(ctx context.Context, subject, op string) error {
	if a.stores.operation(a.issuer, a.key).Phase != registration.EmailSending {
		panic(errSafe)
	}

	a.emails++
	return a.Admin.SendVerification(ctx, subject, op)
}

func combinedBody(n int, email, password string) *strings.Reader {
	data, _ := json.Marshal(
		map[string]string{
			"full_name":     "Synthetic Combined Citizen",
			"nic":           fmt.Sprintf("%09dV", n),
			"mobile_number": "+14155552671",
			"email":         email,
			"password":      password,
		},
	)
	return strings.NewReader(string(data))
}

func combinedKey(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

func serviceCode(err error, want string) bool {
	var e *registration.ServiceError
	return errors.As(err, &e) && e.Code == want
}

func (l *lab) combinedSetup(ctx context.Context) (keycloak.RegistrationConfig, string, string, string) {
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
			"smtpServer":                smtpSettings("1025"),
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
	_, secret, principal := l.createClient(ctx, "lab-combined")
	policyBase := p + "/clients/" + l.clientID(ctx, "admin-permissions") + "/authz/resource-server"
	policy := decode[object](
		l.must(
			ctx,
			"POST",
			policyBase+"/policy/user",
			object{
				"name":  "combined-principal",
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
	rm := l.clientID(ctx, "realm-management")
	roles := decode[[]object](
		l.must(ctx, "GET", p+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite", nil, 200),
	)
	l.lifeCheck("COMBINED_NO_BROAD_ROLES", len(roles) == 0)
	return keycloak.RegistrationConfig{
		RegistrationGroupPath: "/" + registrationGroup,
		BaseURL:               l.base,
		Realm:                 l.realm,
		ClientID:              "lab-combined",
		ClientSecret:          secret,
		VerificationClientID:  "lab-verification",
		VerificationRedirect:  "https://localhost/ga48-verified",
		VerificationLifespan:  time.Minute,
		CallTimeout:           5 * time.Second,
		MaxResponseBytes:      64 << 10,
		TokenSkew:             5 * time.Second,
	}, group, principal, rm
}

func smtpSettings(port string) object {
	return object{
		"host":     "lab-mail",
		"port":     port,
		"from":     "noreply@ga48.test",
		"auth":     "false",
		"ssl":      "false",
		"starttls": "false",
	}
}

func (l *lab) combinedService(cfg keycloak.RegistrationConfig, group string, n int) (*registration.Service, *combinedStores, *combinedAdmin, func()) {
	real, err := keycloak.NewRegistrationAdmin(cfg, l.client)
	if err != nil {
		panic(errSafe)
	}

	d := newCombinedStores()
	issuer := l.base + "/realms/" + l.realm
	d.beforeProfile = func(ctx context.Context, r registration.ProfileRecord) error {
		// Independent observer checks initial real identity at the persistence boundary.
		l.verifyGroupIdentity(ctx, r.Subject, r.OperationID, group)
		return nil
	}
	a := &combinedAdmin{Admin: real, stores: d, issuer: issuer, key: combinedKey(n)}
	fingerprintKey := make([]byte, 32)
	if _, err := rand.Read(fingerprintKey); err != nil {
		panic(errSafe)
	}

	s, err := registration.NewService(
		registration.ServiceConfig{
			Issuer:            issuer,
			FingerprintKey:    fingerprintKey,
			Timeout:           25 * time.Second,
			EmailRetryBackoff: time.Second,
			MaxEmailAttempts:  3,
		},
		a,
		d,
		d,
	)
	if err != nil {
		panic(errSafe)
	}

	return s, d, a, real.Close
}

func (l *lab) realUser(ctx context.Context, subject string) object {
	return decode[object](l.must(ctx, "GET", "/admin/realms/"+l.realm+"/users/"+subject, nil, 200))
}

func (l *lab) realCredentials(ctx context.Context, subject string) any {
	return decode[any](l.must(ctx, "GET", "/admin/realms/"+l.realm+"/users/"+subject+"/credentials", nil, 200))
}

func (l *lab) userCount(ctx context.Context, email string) int {
	return len(
		decode[[]object](
			l.must(
				ctx,
				"GET",
				"/admin/realms/"+l.realm+"/users?email="+url.QueryEscape(email)+"&exact=true",
				nil,
				200,
			),
		),
	)
}

func (l *lab) combinedScenario(ctx context.Context) error {
	fmt.Fprintln(l.out, "COMBINED_STORES SYNTHETIC_NOT_DURABLE")
	fmt.Fprintln(l.out, "COMBINED_GROUP_AUTHORITY ACCEPTED_PROTOTYPE_LIMITATION")
	cfg, group, principal, rm := l.combinedSetup(ctx)
	if err := l.liveTLS(ctx); err != nil {
		return err
	}
	// First registration follows the established no-query-users bootstrap order.
	s, d, a, closeAdmin := l.combinedService(cfg, group, 101)
	defer closeAdmin()
	_, validationErr := s.Register(
		ctx,
		a.key,
		strings.NewReader(
			`{"full_name":"Synthetic","nic":"000000101V","mobile_number":"+14155552671","email":"synthetic@example.test","password":"synthetic-password","groups":["/administrators"],"roles":["admin"]}`,
		),
	)
	var inputErr *registration.RequestError
	l.lifeCheck(
		"COMBINED_VALIDATION_NO_EFFECTS",
		errors.As(validationErr, &inputErr) &&
			inputErr.Code == "invalid_request" &&
			a.creates == 0 &&
			len(d.ops) == 0,
	)
	email := "combined" + randomID() + "@example.test"
	password := randomID()
	_, err := s.Register(ctx, combinedKey(101), combinedBody(101, email, password))
	if err != nil {
		fmt.Fprintln(l.out, "COMBINED_SUCCESS BLOCKED")
		return errSafe
	}

	op := d.operation(a.issuer, a.key)
	u := l.realUser(ctx, op.Subject)
	l.lifeCheck(
		"COMBINED_SUCCESS",
		op.Phase == registration.Completed &&
			op.EmailState == "request_accepted" &&
			u["enabled"] == true &&
			u["emailVerified"] == false &&
			containsAction(u, "VERIFY_EMAIL") &&
			d.hasProfile(a.issuer, op.Subject, op.OperationID) &&
			a.creates == 1 &&
			a.enables == 1 &&
			a.emails == 1,
	)
	if _, err = l.verificationLink(ctx, email); err != nil {
		return err
	}

	l.lifeCheck("COMBINED_MAIL_DELIVERY", true)
	before := l.realCredentials(ctx, op.Subject)
	_, err = s.Register(ctx, a.key, combinedBody(101, email, "different-transient-password"))
	l.lifeCheck(
		"COMBINED_COMPLETED_REPLAY",
		err == nil &&
			a.creates == 1 &&
			a.emails == 1 &&
			a.enables == 1 &&
			d.profileCalls == 1 &&
			l.userCount(ctx, email) == 1 &&
			reflect.DeepEqual(before, l.realCredentials(ctx, op.Subject)) &&
			reflect.DeepEqual(u, l.realUser(ctx, op.Subject)),
	)
	// Collection recovery needs query-users; refresh adapter/client token afterwards.
	p := "/admin/realms/" + l.realm
	l.grant(ctx, principal, rm, "query-users")
	roles := decode[[]object](
		l.must(ctx, "GET", p+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite", nil, 200),
	)
	l.lifeCheck("COMBINED_SCOPED_ROLES", len(roles) == 1 && roles[0]["name"] == "query-users")
	// Simulated profile outage, with a real disabled account and independent readback.
	ps, pd, pa, closeProfile := l.combinedService(cfg, group, 102)
	defer closeProfile()
	pd.failProfile = true
	profileEmail := "profile" + randomID() + "@example.test"
	_, err = ps.Register(ctx, pa.key, combinedBody(102, profileEmail, randomID()))
	po := pd.operation(pa.issuer, pa.key)
	l.lifeCheck(
		"COMBINED_PROFILE_FAILURE_DISABLED",
		serviceCode(err, "registration_unavailable") &&
			po.Phase == registration.SubjectRecorded &&
			l.realUser(ctx, po.Subject)["enabled"] == false &&
			pa.enables == 0 &&
			pa.emails == 0 &&
			!pd.hasProfile(pa.issuer, po.Subject, po.OperationID),
	)
	profileCreds := l.realCredentials(ctx, po.Subject)
	pd.failProfile = false
	_, err = ps.Register(ctx, pa.key, combinedBody(102, profileEmail, "retry-cannot-overwrite-password"))
	l.lifeCheck(
		"COMBINED_PROFILE_RETRY",
		err == nil &&
			pa.creates == 1 &&
			pd.profileCalls == 2 &&
			pa.enables == 1 &&
			pa.emails == 1 &&
			l.userCount(ctx, profileEmail) == 1 &&
			reflect.DeepEqual(profileCreds, l.realCredentials(ctx, po.Subject)),
	)
	// Successful real create, result discarded once at the adapter boundary.
	as, ad, aa, closeAmbiguous := l.combinedService(cfg, group, 103)
	defer closeAmbiguous()
	aa.loseCreateResult = true
	ambiguousEmail := "ambiguous" + randomID() + "@example.test"
	_, err = as.Register(ctx, aa.key, combinedBody(103, ambiguousEmail, randomID()))
	ao := ad.operation(aa.issuer, aa.key)
	l.lifeCheck(
		"COMBINED_CREATE_RESULT_LOST",
		serviceCode(err, "registration_reconciliation_required") &&
			ao.Phase == registration.Creating &&
			ao.Subject == "" &&
			aa.creates == 1 &&
			aa.enables == 0 &&
			l.userCount(ctx, ambiguousEmail) == 1,
	)
	ambiguousUsers := decode[[]object](
		l.must(ctx, "GET", p+"/users?email="+url.QueryEscape(ambiguousEmail)+"&exact=true", nil, 200),
	)
	if len(ambiguousUsers) != 1 {
		panic(errSafe)
	}

	ambiguousCredentials := l.realCredentials(ctx, ambiguousUsers[0]["id"].(string))
	_, err = as.Resume(ctx, aa.key)
	ao = ad.operation(aa.issuer, aa.key)
	l.lifeCheck(
		"COMBINED_MARKER_RECONCILIATION",
		err == nil &&
			ao.Phase == registration.Completed &&
			aa.creates == 1 &&
			aa.finds == 1 &&
			aa.enables == 1 &&
			aa.emails == 1 &&
			l.userCount(ctx, ambiguousEmail) == 1,
	)
	l.lifeCheck(
		"COMBINED_RECONCILE_CREDENTIALS_UNCHANGED",
		reflect.DeepEqual(ambiguousCredentials, l.realCredentials(ctx, ao.Subject)),
	)
	// Real SMTP connection failure, affecting only this disposable realm.
	l.must(ctx, "PUT", p, object{"smtpServer": smtpSettings("1")}, 204)
	es, ed, ea, closeEmail := l.combinedService(cfg, group, 104)
	defer closeEmail()
	failureEmail := "emailfailure" + randomID() + "@example.test"
	_, err = es.Register(ctx, ea.key, combinedBody(104, failureEmail, randomID()))
	eo := ed.operation(ea.issuer, ea.key)
	l.lifeCheck(
		"COMBINED_EMAIL_FAILURE_PENDING",
		serviceCode(err, "registration_reconciliation_required") &&
			eo.Phase == registration.EmailSending &&
			ea.creates == 1 &&
			ea.emails == 1 &&
			l.realUser(ctx, eo.Subject)["enabled"] == true &&
			l.realUser(ctx, eo.Subject)["emailVerified"] == false &&
			ed.hasProfile(ea.issuer, eo.Subject, eo.OperationID),
	)
	l.must(ctx, "PUT", p, object{"smtpServer": smtpSettings("1025")}, 204)
	_, err = es.Resume(ctx, ea.key)
	l.lifeCheck(
		"COMBINED_UNCERTAIN_EMAIL_NOT_RESENT",
		serviceCode(err, "registration_reconciliation_required") &&
			ea.emails == 1 &&
			ea.creates == 1 &&
			l.userCount(ctx, failureEmail) == 1,
	)
	var inbox struct {
		Messages []struct{ To []struct{ Address string } }
	}
	if err = l.mailJSON(ctx, "/api/v1/messages", &inbox); err != nil {
		return err
	}

	noFailedEmail := true
	for _, m := range inbox.Messages {
		for _, to := range m.To {
			if to.Address == failureEmail {
				noFailedEmail = false
			}
		}
	}

	l.lifeCheck("COMBINED_FAILED_EMAIL_ABSENT", noFailedEmail)
	return nil
}
