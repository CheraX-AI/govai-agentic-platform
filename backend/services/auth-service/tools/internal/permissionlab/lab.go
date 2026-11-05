// Package permissionlab contains opt-in infrastructure and permission probes only.
// It is never imported by the API. All remote URLs are derived from a newly
// created, labelled container's loopback port; no caller-supplied Keycloak URL.
package permissionlab

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

const image = "quay.io/keycloak/keycloak:26.8.1"

const label = "govai.ga48.permission-lab"

const mailImage = "axllent/mailpit@sha256:0790692be935ea4420d085694f7985b1f692ca6a8771ab0e0bd8ba75ac180498"

type object = map[string]any

var errSafe = errors.New("lab operation failed")

type lab struct {
	client                       *http.Client
	base, realm, other, observer string
	out                          io.Writer
	unsafe                       bool
	mailBase                     string
	mailSMTPPort                 int
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(errSafe)
	}

	return hex.EncodeToString(b)
}

// Run returns 0 only if all functional and security assertions pass, 2 for a
// completed experiment with inadequate privilege isolation, 1 for blocked setup.
// It never deletes resources or prints remote errors, bodies or credentials.
func Run(ctx context.Context, stateDir string, out io.Writer) (code int) {
	return execute(ctx, stateDir, out, "baseline")
}

// RunGroups opts into a separate group-scoped permission experiment.
func RunGroups(ctx context.Context, stateDir string, out io.Writer) int {
	return execute(ctx, stateDir, out, "groups")
}

// RunLifecycle does not change the historical permission-isolation assertions.
func RunLifecycle(ctx context.Context, stateDir string, out io.Writer) int {
	return execute(ctx, stateDir, out, "lifecycle")
}

func execute(ctx context.Context, stateDir string, out io.Writer, mode string) (code int) {
	code = 1
	defer func() {
		if recover() != nil {
			fmt.Fprintln(out, "LAB_INTERNAL BLOCKED")
		}
	}()
	if err := run(ctx, stateDir, out, mode); err != nil {
		var isolation isolationFailure
		if errors.As(err, &isolation) {
			return 2
		}

		fmt.Fprintln(out, "LAB_EXECUTION BLOCKED")
		return 1
	}

	return 0
}

func docker(ctx context.Context, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = append(os.Environ(), env...)
	data, err := cmd.Output() // stderr is captured by os/exec, never forwarded
	if err != nil {
		return nil, errSafe
	}

	return data, nil
}

func run(ctx context.Context, stateDir string, out io.Writer, mode string) error {
	return runWithScenario(ctx, stateDir, out, mode, nil)
}

// Scenario injection is used only by opt-in integration tests. Synthetic stores
// are defined in _test.go files and are never linked into the application/CLI.
func runWithScenario(
	ctx context.Context,
	stateDir string,
	out io.Writer,
	mode string,
	scenario func(context.Context, *lab) error,
) error {
	stateDir, err := filepath.Abs(stateDir)
	if err != nil || insideGit(stateDir) {
		return errSafe
	}
	// Require a new directory: never overwrite a previous lab or arbitrary files.
	if err = os.Mkdir(stateDir, 0700); err != nil {
		return errSafe
	}

	ca, err := certificates(stateDir)
	if err != nil {
		return errSafe
	}

	id := randomID()
	name, volume := "ga48-permission-"+id, "ga48-permission-data-"+id
	network := "ga48-permission-network-" + id
	secret := randomID() + randomID()
	if _, err = docker(ctx, nil, "volume", "create", "--label", label+"="+id, volume); err != nil {
		fmt.Fprintln(out, "LAB_DOCKER BLOCKED")
		return err
	}

	networkArgs := []string{"network", "create", "--label", label + "=" + id}
	if mode == "recovery" {
		// Retained historical /16 test networks can exhaust Docker's default
		// pool. A small explicit bridge subnet avoids deleting old resources.
		octets, err := hex.DecodeString(id[:4])
		if err != nil {
			return errSafe
		}

		networkArgs = append(networkArgs, "--subnet", fmt.Sprintf("10.252.%d.%d/28", octets[0], octets[1]&240))
	}

	networkArgs = append(networkArgs, network)
	if _, err = docker(ctx, nil, networkArgs...); err != nil {
		fmt.Fprintln(out, "LAB_NETWORK BLOCKED")
		return err
	}

	mailName, mailVolume, mailBase := "", "", ""
	mailSMTPPort := 0
	if mode != "baseline" {
		mailName, mailVolume = "ga48-permission-mail-"+id, "ga48-permission-mail-data-"+id
		if _, err = docker(ctx, nil, "volume", "create", "--label", label+"="+id, mailVolume); err != nil {
			return err
		}

		mailArgs := []string{
			"run",
			"-d",
			"--name",
			mailName,
			"--label",
			label + "=" + id,
			"--network",
			network,
			"--network-alias",
			"lab-mail",
			"--publish",
			"127.0.0.1::8025",
			"--mount",
			"type=volume,source=" + mailVolume + ",target=/data",
			"--env",
			"MP_DATABASE=/data/mailpit.db",
			"--env",
			"MP_QUIET=true",
		}
		if mode == "recovery" {
			mailArgs = append(mailArgs, "--publish", "127.0.0.1::1025")
		}

		mailArgs = append(mailArgs, mailImage)
		if _, err = docker(ctx, nil, mailArgs...); err != nil {
			return err
		}

		defer stopContainer(out, mailName)
		mp, err := docker(
			ctx,
			nil,
			"inspect",
			"--format",
			`{{(index (index .NetworkSettings.Ports "8025/tcp") 0).HostPort}}`,
			mailName,
		)
		if err != nil {
			return err
		}

		port, err := strconv.Atoi(strings.TrimSpace(string(mp)))
		if err != nil || port < 1 || port > 65535 {
			return errSafe
		}

		mailBase = fmt.Sprintf("http://127.0.0.1:%d", port)
		if mode == "recovery" {
			sp, err := docker(
				ctx,
				nil,
				"inspect",
				"--format",
				`{{(index (index .NetworkSettings.Ports "1025/tcp") 0).HostPort}}`,
				mailName,
			)
			if err != nil {
				return errSafe
			}

			mailSMTPPort, err = strconv.Atoi(strings.TrimSpace(string(sp)))
			if err != nil || mailSMTPPort < 1 || mailSMTPPort > 65535 {
				return errSafe
			}
		}
	}
	// Separate bridge network/storage; Mailpit exists only for opted-in modes.
	args := []string{
		"run",
		"-d",
		"--name",
		name,
		"--label",
		label + "=" + id,
		"--network",
		network,
		"--publish",
		"127.0.0.1::8443",
		"--mount",
		"type=volume,source=" + volume + ",target=/opt/keycloak/data",
		"--mount",
		"type=bind,source=" + stateDir + ",target=/lab,readonly",
		"--env",
		"KC_BOOTSTRAP_ADMIN_CLIENT_ID=lab-observer",
		"--env",
		"KC_BOOTSTRAP_ADMIN_CLIENT_SECRET",
		image,
		"start-dev",
		"--http-enabled=false",
		"--hostname-strict=false",
		"--https-certificate-file=/lab/server.pem",
		"--https-certificate-key-file=/lab/server-key.pem",
		"--features=admin-fine-grained-authz:v2",
		"--log-level=OFF",
		"--http-access-log-enabled=false",
	}
	if _, err = docker(ctx, []string{"KC_BOOTSTRAP_ADMIN_CLIENT_SECRET=" + secret}, args...); err != nil {
		fmt.Fprintln(out, "LAB_CONTAINER BLOCKED")
		return err
	}

	defer stopContainer(out, name)
	ports, err := docker(
		ctx,
		nil,
		"inspect",
		"--format",
		`{{(index (index .NetworkSettings.Ports "8443/tcp") 0).HostPort}}`,
		name,
	)
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(strings.TrimSpace(string(ports)))
	if err != nil || port < 1 || port > 65535 {
		return errSafe
	}

	manifest, _ := json.MarshalIndent(
		object{
			"container":      name,
			"volume":         volume,
			"network":        network,
			"label":          id,
			"image":          image,
			"base_url":       fmt.Sprintf("https://localhost:%d", port),
			"ca":             "ca.pem",
			"mail_container": mailName,
			"mail_volume":    mailVolume,
			"mail_url":       mailBase,
		},
		"",
		"  ",
	)
	if os.WriteFile(filepath.Join(stateDir, "resources.json"), manifest, 0600) != nil {
		return errSafe
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return errSafe
	}

	transport := &http.Transport{
		TLSClientConfig:        &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		MaxResponseHeaderBytes: 16 << 10,
	}
	defer transport.CloseIdleConnections()
	l := &lab{
		base:  fmt.Sprintf("https://localhost:%d", port),
		realm: "ga48-permission-" + id,
		other: "ga48-other-" + id,
		out:   out,
		client: &http.Client{
			Transport:     transport,
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	l.mailBase = mailBase
	l.mailSMTPPort = mailSMTPPort
	ready := false
	for i := 0; i < 120; i++ {
		token, e := l.token(ctx, "master", "lab-observer", secret)
		if e == nil {
			l.observer = token
			ready = true
			break
		}

		select {
		case <-ctx.Done():
			return errSafe
		case <-time.After(time.Second):
		}
	}

	if !ready {
		fmt.Fprintln(out, "LAB_READY BLOCKED")
		return errSafe
	}

	fmt.Fprintln(out, "LAB_TLS PASS")
	if scenario != nil {
		err = scenario(ctx, l)
	} else if mode == "lifecycle" {
		err = l.lifecycle(ctx)
	} else if mode == "groups" {
		err = l.groupExperiment(ctx)
	} else {
		err = l.experiment(ctx)
	}

	if err != nil {
		return err
	}

	if mode == "combined" || mode == "recovery" {
		if l.unsafe {
			fmt.Fprintln(out, "LAB_COMBINED FAIL")
			return isolationFailure{}
		}

		fmt.Fprintln(out, "LAB_COMBINED PASS")
		return nil
	}

	if mode == "lifecycle" {
		if l.unsafe {
			fmt.Fprintln(out, "LAB_LIFECYCLE FAIL")
			return isolationFailure{}
		}

		fmt.Fprintln(out, "LAB_LIFECYCLE PASS")
		return nil
	}

	if l.unsafe {
		fmt.Fprintln(out, "LAB_RELEASE FAIL_UNSAFE")
		return isolationFailure{}
	}

	fmt.Fprintln(out, "LAB_RELEASE PASS")
	return nil
}

func stopContainer(out io.Writer, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := docker(ctx, nil, "stop", "--timeout", "10", name); err != nil {
		fmt.Fprintln(out, "LAB_STOP BLOCKED")
	} else {
		fmt.Fprintln(out, "LAB_STOP PASS")
	}
}

func insideGit(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}

		dir = parent
	}
}

type isolationFailure struct{}

func (isolationFailure) Error() string { return "inadequate isolation" }

func certificates(dir string) ([]byte, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errSafe
	}

	now := time.Now()
	serial := func() *big.Int {
		n, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
		if e != nil {
			panic(errSafe)
		}

		return n
	}
	ca := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "GA48 disposable test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, errSafe
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errSafe
	}

	leaf := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, errSafe
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, errSafe
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	for name, data := range map[string][]byte{
		"ca.pem":         caPEM,
		"server.pem":     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		"server-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	} {
		if os.WriteFile(filepath.Join(dir, name), data, 0600) != nil {
			return nil, errSafe
		}
	}

	return caPEM, nil // CA signing key never leaves memory
}

// request is a one-shot bounded call. All response values remain private.
func (l *lab) request(ctx context.Context, token, method, path string, body any) (int, []byte, error) {
	if !allowedPath(path, l.realm, l.other) {
		return 0, nil, errSafe
	}

	var reader io.Reader
	ct := "application/json"
	if v, ok := body.(url.Values); ok {
		reader = strings.NewReader(v.Encode())
		ct = "application/x-www-form-urlencoded"
	} else if body != nil {
		data, e := json.Marshal(body)
		if e != nil {
			return 0, nil, errSafe
		}

		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, l.base+path, reader)
	if err != nil {
		return 0, nil, errSafe
	}

	req.GetBody = nil
	req.Header.Set("Content-Type", ct)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	r, err := l.client.Do(req)
	if err != nil {
		return 0, nil, errSafe
	}

	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return r.StatusCode, nil, errSafe
	}

	return r.StatusCode, data, nil
}

var realmName = regexp.MustCompile(`^ga48-(permission|other)-[a-f0-9]{32}$`)

func allowedPath(path, realm, other string) bool {
	if !realmName.MatchString(realm) ||
		!realmName.MatchString(other) ||
		strings.ContainsAny(path, "\\#") ||
		strings.Contains(path, "..") {
		return false
	}

	if path == "/admin/realms" || path == "/realms/master/protocol/openid-connect/token" {
		return true
	}

	for _, r := range []string{realm, other} {
		if strings.HasPrefix(path, "/admin/realms/"+r+"/") ||
			path == "/admin/realms/"+r ||
			path == "/realms/"+r+"/protocol/openid-connect/token" {
			return true
		}
	}

	return false
}

func (l *lab) token(ctx context.Context, realm, client, secret string) (string, error) {
	status, data, err := l.request(
		ctx,
		"",
		"POST",
		"/realms/"+realm+"/protocol/openid-connect/token",
		url.Values{"grant_type": {"client_credentials"}, "client_id": {client}, "client_secret": {secret}},
	)
	if err != nil || status != 200 {
		return "", errSafe
	}

	var v struct {
		Token string `json:"access_token"`
	}
	if json.Unmarshal(data, &v) != nil || v.Token == "" {
		return "", errSafe
	}

	return v.Token, nil
}

func (l *lab) must(ctx context.Context, method, path string, body any, status int) []byte {
	s, data, err := l.request(ctx, l.observer, method, path, body)
	if err != nil || s != status {
		panic(errSafe)
	}

	return data
}

func decode[T any](data []byte) T {
	var v T
	if json.Unmarshal(data, &v) != nil {
		panic(errSafe)
	}

	return v
}

func (l *lab) clientID(ctx context.Context, name string) string {
	data := l.must(ctx, "GET", "/admin/realms/"+l.realm+"/clients?clientId="+url.QueryEscape(name), nil, 200)
	rows := decode[[]object](data)
	if len(rows) != 1 {
		panic(errSafe)
	}

	return rows[0]["id"].(string)
}

func (l *lab) createClient(ctx context.Context, name string) (string, string, string) {
	secret := randomID() + randomID()
	l.must(
		ctx,
		"POST",
		"/admin/realms/"+l.realm+"/clients",
		object{
			"clientId":                  name,
			"secret":                    secret,
			"protocol":                  "openid-connect",
			"publicClient":              false,
			"serviceAccountsEnabled":    true,
			"standardFlowEnabled":       false,
			"directAccessGrantsEnabled": false,
			"implicitFlowEnabled":       false,
			"fullScopeAllowed":          true,
		},
		201,
	)
	id := l.clientID(ctx, name)
	user := decode[object](
		l.must(ctx, "GET", "/admin/realms/"+l.realm+"/clients/"+id+"/service-account-user", nil, 200),
	)
	return id, secret, user["id"].(string)
}

func (l *lab) grant(ctx context.Context, subject, client, name string) {
	p := "/admin/realms/" + l.realm
	role := decode[object](l.must(ctx, "GET", p+"/clients/"+client+"/roles/"+name, nil, 200))
	l.must(ctx, "POST", p+"/users/"+subject+"/role-mappings/clients/"+client, []object{role}, 204)
}

func (l *lab) user(ctx context.Context, name string, admin bool) string {
	p := "/admin/realms/" + l.realm
	l.must(
		ctx,
		"POST",
		p+"/users",
		object{
			"username":      name,
			"email":         name + "@example.test",
			"enabled":       true,
			"emailVerified": false,
			"credentials":   []object{{"type": "password", "value": randomID(), "temporary": false}},
		},
		201,
	)
	rows := decode[[]object](l.must(ctx, "GET", p+"/users?username="+name+"&exact=true", nil, 200))
	if len(rows) != 1 {
		panic(errSafe)
	}

	id := rows[0]["id"].(string)
	if admin {
		l.grant(ctx, id, l.clientID(ctx, "realm-management"), "realm-admin")
	}

	return id
}

func (l *lab) result(id, result string) {
	fmt.Fprintln(l.out, id, result)
	if result != "PASS" {
		l.unsafe = true
	}
}

func (l *lab) experiment(ctx context.Context) error {
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
	_, broadSecret, broadSubject := l.createClient(ctx, "lab-broad")
	_, fineSecret, fineSubject := l.createClient(ctx, "lab-fine")
	rm := l.clientID(ctx, "realm-management")
	l.grant(ctx, broadSubject, rm, "manage-users")
	// Fine-grained v2: all-user view/manage/reset-password, no map-roles.
	permissions := l.clientID(ctx, "admin-permissions")
	policyBase := p + "/clients/" + permissions + "/authz/resource-server"
	policy := decode[object](
		l.must(
			ctx,
			"POST",
			policyBase+"/policy/user",
			object{
				"name":  "lab-fine-principal",
				"type":  "user",
				"logic": "POSITIVE",
				"users": []string{fineSubject},
			},
			201,
		),
	)
	permission := decode[object](
		l.must(
			ctx,
			"POST",
			policyBase+"/permission/scope",
			object{
				"name":             "lab-fine-users",
				"type":             "scope",
				"resourceType":     "Users",
				"scopes":           []string{"view", "manage", "reset-password"},
				"policies":         []string{policy["id"].(string)},
				"decisionStrategy": "UNANIMOUS",
			},
			201,
		),
	)
	fmt.Fprintln(l.out, "LAB_SETUP PASS")
	for _, c := range []struct{ id, name, secret string }{{"BASE", "lab-broad", broadSecret}, {"FINE", "lab-fine", fineSecret}} {
		token, err := l.token(ctx, l.realm, c.name, c.secret)
		if err != nil {
			return err
		}

		cfg := keycloak.RegistrationConfig{
			RegistrationGroupPath: "/" + registrationGroup,
			BaseURL:               l.base,
			Realm:                 l.realm,
			ClientID:              c.name,
			ClientSecret:          c.secret,
			VerificationClientID:  "intentionally-missing",
			VerificationRedirect:  "https://localhost/unused",
			VerificationLifespan:  time.Minute,
			CallTimeout:           5 * time.Second,
			MaxResponseBytes:      64 << 10,
			TokenSkew:             5 * time.Second,
		}
		a, err := keycloak.NewRegistrationAdmin(cfg, l.client)
		if err != nil {
			return err
		}

		raw := fmt.Sprintf(
			`{"full_name":"Synthetic Test","nic":"000000001V","mobile_number":"+14155552671","email":"%s@example.test","password":"%s"}`,
			strings.ToLower(c.id)+randomID(),
			randomID(),
		)
		r, e := registration.ParseRequest(strings.NewReader(raw))
		if e != nil {
			return errSafe
		}

		op := randomID()
		// Keep the historical baseline group-free; current adapter create is scoped.
		u, err := l.baselineCreate(ctx, token, r, op)
		if err != nil {
			a.Close()
			l.result(c.id+"_CREATE", "FAIL_REQUIRED")
			continue
		}

		observed := decode[object](l.must(ctx, "GET", p+"/users/"+u.Subject, nil, 200))
		if observed["enabled"] != false || observed["emailVerified"] != false {
			panic(errSafe)
		}

		l.result(c.id+"_CREATE", "PASS")
		found, err := a.FindOwned(ctx, op)
		if err != nil || found.Subject != u.Subject {
			panic(errSafe)
		}

		if _, err = a.ReadOwned(ctx, u.Subject, op); err != nil {
			panic(errSafe)
		}

		l.result(c.id+"_RECONCILE", "PASS")
		// Synthetic permission probe only; no application profile completion claim.
		if err = a.EnableOwned(ctx, u.Subject, op); err != nil {
			panic(errSafe)
		}

		observed = decode[object](l.must(ctx, "GET", p+"/users/"+u.Subject, nil, 200))
		if observed["enabled"] != true {
			panic(errSafe)
		}

		l.result(c.id+"_ENABLE", "PASS")
		// An invalid action client tests the manage gate without SMTP or email.
		status, _, err := l.request(
			ctx,
			token,
			"PUT",
			p+"/users/"+u.Subject+"/send-verify-email?client_id=intentionally-missing",
			nil,
		)
		if err != nil || status != 400 {
			panic(errSafe)
		}

		l.result(c.id+"_EMAIL_PERMISSION_GATE", "PASS")
		a.Close()
		for _, target := range []struct {
			kind  string
			admin bool
		}{{"UNRELATED", false}, {"ADMIN", true}} {
			subject := l.user(ctx, strings.ToLower(c.id+target.kind)+randomID(), target.admin)
			l.probeUpdate(ctx, c.id+"_"+target.kind, token, subject)
			l.probePassword(ctx, c.id+"_"+target.kind, token, subject)
		}

		l.probeRoles(ctx, c.id, token, u.Subject, rm)
		status, _, err = l.request(ctx, token, "GET", "/admin/realms/"+l.other+"/users", nil)
		if err != nil {
			panic(errSafe)
		}

		if status == 403 {
			l.result(c.id+"_CROSS_REALM", "PASS")
		} else {
			l.result(c.id+"_CROSS_REALM", "FAIL_UNSAFE")
		}

		crossName := "cross" + randomID()
		status, _, err = l.request(
			ctx,
			token,
			"POST",
			"/admin/realms/"+l.other+"/users",
			object{"username": crossName, "enabled": false},
		)
		if err != nil {
			panic(errSafe)
		}

		crossRows := decode[[]object](
			l.must(ctx, "GET", "/admin/realms/"+l.other+"/users?username="+crossName+"&exact=true", nil, 200),
		)
		if status == 403 && len(crossRows) == 0 {
			l.result(c.id+"_CROSS_REALM_CREATE", "PASS")
		} else {
			l.result(c.id+"_CROSS_REALM_CREATE", "FAIL_UNSAFE")
		}
		// Tighten the fine candidate to explicit owned-user resources, keeping
		// identical security assertions; creation should become unavailable.
		if c.id == "FINE" {
			permission["resources"] = []string{u.Subject}
			permission["resourceType"] = "Users"
			permission["scopes"] = []string{"view", "manage", "reset-password"}
			permission["policies"] = []string{policy["id"].(string)}
			l.must(ctx, "PUT", policyBase+"/permission/scope/"+permission["id"].(string), permission, 201)
			status, _, err = l.request(
				ctx,
				token,
				"POST",
				p+"/users",
				object{
					"username":    "scoped" + randomID(),
					"email":       "scoped" + randomID() + "@example.test",
					"enabled":     false,
					"credentials": []object{{"type": "password", "value": randomID(), "temporary": false}},
				},
			)
			if err != nil {
				panic(errSafe)
			}

			if status == 201 {
				l.result("SCOPED_CREATE", "PASS")
			} else if status == 403 {
				l.result("SCOPED_CREATE", "FAIL_REQUIRED")
			} else {
				panic(errSafe)
			}

			subject := l.user(ctx, "scopedunrelated"+randomID(), false)
			l.probeUpdate(ctx, "SCOPED_UNRELATED", token, subject)
			l.probePassword(ctx, "SCOPED_UNRELATED", token, subject)
			adminSubject := l.user(ctx, "scopedadmin"+randomID(), true)
			l.probeUpdate(ctx, "SCOPED_ADMIN", token, adminSubject)
			l.probePassword(ctx, "SCOPED_ADMIN", token, adminSubject)
			l.probeRoles(ctx, "SCOPED", token, u.Subject, rm)
		}
	}

	return nil
}

func (l *lab) probeUpdate(ctx context.Context, id, token, subject string) {
	p := "/admin/realms/" + l.realm + "/users/" + subject
	before := decode[object](l.must(ctx, "GET", p, nil, 200))
	status, _, err := l.request(ctx, token, "PUT", p, object{"enabled": false})
	if err != nil {
		panic(errSafe)
	}

	after := decode[object](l.must(ctx, "GET", p, nil, 200))
	if status == 403 && before["enabled"] == after["enabled"] {
		l.result(id+"_UPDATE", "PASS")
	} else if status == 204 && after["enabled"] == false {
		l.result(id+"_UPDATE", "FAIL_UNSAFE")
	} else {
		panic(errSafe)
	}
}

func (l *lab) probePassword(ctx context.Context, id, token, subject string) {
	p := "/admin/realms/" + l.realm + "/users/" + subject
	before := l.must(ctx, "GET", p+"/credentials", nil, 200)
	status, _, err := l.request(
		ctx,
		token,
		"PUT",
		p+"/reset-password",
		object{"type": "password", "value": randomID(), "temporary": false},
	)
	if err != nil {
		panic(errSafe)
	}

	after := l.must(ctx, "GET", p+"/credentials", nil, 200)
	// Credential metadata is observed privately; no password grant is used.
	if status == 403 && bytes.Equal(before, after) {
		l.result(id+"_PASSWORD", "PASS")
	} else if status == 204 && !bytes.Equal(before, after) {
		l.result(id+"_PASSWORD", "FAIL_UNSAFE")
	} else {
		panic(errSafe)
	}
}

func (l *lab) probeRoles(ctx context.Context, id, token, subject, rm string) {
	p := "/admin/realms/" + l.realm
	// Actual realm-management privileges, assigned only to a synthetic account.
	for _, name := range []string{"realm-admin", "manage-users"} {
		role := decode[object](l.must(ctx, "GET", p+"/clients/"+rm+"/roles/"+name, nil, 200))
		path := p + "/users/" + subject + "/role-mappings/clients/" + rm
		status, _, err := l.request(ctx, token, "POST", path, []object{role})
		if err != nil {
			panic(errSafe)
		}

		roles := decode[[]object](l.must(ctx, "GET", path, nil, 200))
		has := false
		for _, r := range roles {
			if r["id"] == role["id"] {
				has = true
			}
		}

		if status == 403 && !has {
			l.result(id+"_ROLE_"+strings.ToUpper(strings.ReplaceAll(name, "-", "_")), "PASS")
		} else if status == 204 && has {
			l.result(id+"_ROLE_"+strings.ToUpper(strings.ReplaceAll(name, "-", "_")), "FAIL_UNSAFE")
		} else {
			panic(errSafe)
		}
	}
}
