package permissionlab

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/app/wiring"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Explicitly separate from TestCombinedLive: that historical test continues to
// use synthetic stores and retains its original evidence and verdicts.
func TestPostgresKeycloakLive(t *testing.T) {
	if os.Getenv("GA48_POSTGRES_LIVE") != "1" {
		t.Skip("real PostgreSQL/Keycloak opt-in disabled")
	}

	state := os.Getenv("GA48_POSTGRES_STATE_DIR")
	if state == "" {
		t.Fatal("POSTGRES_LIVE_STATE BLOCKED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	err := guardedCombined(func() error {
		return runWithScenario(ctx, state, os.Stdout, "combined", func(ctx context.Context, l *lab) error {
			return l.postgresScenario(ctx, filepath.Join(state, "ca.pem"))
		})
	}, os.Stdout)
	if err != nil {
		t.Fatal("POSTGRES_KEYCLOAK FAIL_OR_BLOCKED")
	}

	if !safeRegistrationRuntimeLogs(logs.Bytes()) {
		t.Fatal("POSTGRES_KEYCLOAK_LOGS FAIL")
	}

	fmt.Println("POSTGRES_KEYCLOAK_LOGS PASS")
}

// The production router logs only fixed request metadata, never request bodies.
func safeRegistrationRuntimeLogs(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	count := 0
	for {
		var entry map[string]any
		e := decoder.Decode(&entry)
		if e == io.EOF {
			return count == 1
		}

		if e != nil {
			return false
		}

		allowed := map[string]bool{
			"time":        true,
			"level":       true,
			"msg":         true,
			"request_id":  true,
			"method":      true,
			"route":       true,
			"status":      true,
			"duration_ms": true,
		}
		if len(entry) != len(allowed) {
			return false
		}

		for k := range entry {
			if !allowed[k] {
				return false
			}
		}

		if entry["msg"] != "http request" ||
			entry["level"] != "INFO" ||
			entry["route"] != "/api/v1/registrations" ||
			entry["method"] != "POST" ||
			entry["status"] != float64(202) {
			return false
		}

		count++
	}
}

type durableLab struct {
	pool                            *pgxpool.Pool
	admin                           *pgxpool.Pool
	owner                           *pgxpool.Pool
	store                           *postgres.Store
	name, dsn                       string
	dataKey, nicKey, fingerprintKey []byte
}

func freshKey() []byte {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(errSafe)
	}

	return b
}

func newDurableLab(ctx context.Context) (d *durableLab, err error) {
	runtime, e := godotenv.Read("../../.env")
	if e != nil {
		return nil, errSafe
	}

	secrets, e := godotenv.Read("../../.env.migrate")
	if e != nil {
		return nil, errSafe
	}

	ownerURL, e := url.Parse(secrets["MIGRATION_DATABASE_URL"])
	if e != nil || ownerURL.Hostname() != "127.0.0.1" || ownerURL.Port() != "55432" {
		return nil, errSafe
	}

	appURL, e := url.Parse(runtime["DATABASE_URL"])
	if e != nil || appURL.Hostname() != ownerURL.Hostname() || appURL.Port() != ownerURL.Port() {
		return nil, errSafe
	}

	d = &durableLab{
		name:           "govai_auth_live_" + randomID(),
		dataKey:        freshKey(),
		nicKey:         freshKey(),
		fingerprintKey: freshKey(),
	}
	d.owner, e = postgres.Open(ctx, ownerURL.String())
	if e != nil {
		return nil, errSafe
	}

	created := false
	defer func() {
		if err != nil {
			if created {
				_ = d.close()
			} else {
				d.owner.Close()
			}
		}
	}()
	quoted := pgx.Identifier{d.name}.Sanitize()
	if _, e = d.owner.Exec(ctx, "CREATE DATABASE "+quoted); e != nil {
		return d, errSafe
	}

	created = true
	ownerURL.Path = "/" + d.name
	d.admin, e = postgres.Open(ctx, ownerURL.String())
	if e != nil {
		return d, errSafe
	}

	if postgres.Migrate(ctx, d.admin) != nil {
		return d, errSafe
	}

	if _, e = d.admin.Exec(
		ctx,
		"REVOKE ALL ON DATABASE "+quoted+" FROM PUBLIC; GRANT CONNECT ON DATABASE "+quoted+" TO govai_auth_app; REVOKE ALL ON SCHEMA public FROM PUBLIC",
	); e != nil {
		return d, errSafe
	}

	appURL.Path = "/" + d.name
	d.dsn = appURL.String()
	if d.reopen(ctx) != nil {
		return d, errSafe
	}

	return d, nil
}

func (d *durableLab) reopen(ctx context.Context) error {
	if d.pool != nil {
		d.pool.Close()
		d.pool = nil
	}

	p, e := postgres.Open(ctx, d.dsn)
	if e != nil {
		return errSafe
	}

	d.pool = p
	if postgres.Ready(ctx, p) != nil {
		return errSafe
	}

	d.store, e = postgres.NewStore(p, d.dataKey, d.nicKey)
	if e != nil {
		return errSafe
	}

	return nil
}

func (d *durableLab) close() error {
	if d.pool != nil {
		d.pool.Close()
	}

	if d.admin != nil {
		d.admin.Close()
	}
	// Delete ONLY the freshly created synthetic database, never govai_auth.
	if !strings.HasPrefix(d.name, "govai_auth_live_") || len(d.name) != 48 {
		d.owner.Close()
		return errSafe
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, e := d.owner.Exec(ctx, "DROP DATABASE "+pgx.Identifier{d.name}.Sanitize()+" WITH (FORCE)")
	d.owner.Close()
	if e != nil {
		return errSafe
	}

	return nil
}

func (d *durableLab) operation(ctx context.Context, issuer, key string) registration.Operation {
	session, e := d.store.Open(ctx, issuer, key)
	if e != nil {
		panic(errSafe)
	}

	o := session.Snapshot()
	cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if session.Close(cleanup) != nil {
		panic(errSafe)
	}

	return o
}

func (d *durableLab) phase(ctx context.Context, issuer, key string) registration.Phase {
	var phase registration.Phase
	if d.pool.QueryRow(
		ctx,
		"SELECT phase FROM auth.registration_operations WHERE issuer=$1 AND idempotency_key=$2::uuid",
		issuer,
		key,
	).Scan(&phase) != nil {
		panic(errSafe)
	}

	return phase
}

func (d *durableLab) profile(ctx context.Context, issuer, subject, op string) bool {
	var present bool
	if d.pool.QueryRow(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM auth.citizen_profiles WHERE issuer=$1 AND subject=$2 AND operation_id=$3)",
		issuer,
		subject,
		op,
	).Scan(&present) != nil {
		panic(errSafe)
	}

	return present
}

type durableAdmin struct {
	registration.Admin
	db                              *durableLab
	issuer, key                     string
	creates, enables, emails, finds int
	discardCreate                   bool
}

func (a *durableAdmin) CreateDisabled(ctx context.Context, r registration.Request, op string) (registration.Identity, error) {
	if a.db.phase(ctx, a.issuer, a.key) != registration.Creating {
		panic(errSafe)
	}

	a.creates++
	u, e := a.Admin.CreateDisabled(ctx, r, op)
	if e == nil && a.discardCreate {
		a.discardCreate = false
		return registration.Identity{}, &registration.AdminError{Code: registration.AdminAmbiguous, UnknownOutcome: true}
	}

	return u, e
}

func (a *durableAdmin) FindOwned(ctx context.Context, op string) (registration.Identity, error) {
	a.finds++
	return a.Admin.FindOwned(ctx, op)
}

func (a *durableAdmin) EnableOwned(ctx context.Context, subject, op string) error {
	if a.db.phase(ctx, a.issuer, a.key) != registration.Enabling ||
		!a.db.profile(ctx, a.issuer, subject, op) {
		panic(errSafe)
	}

	a.enables++
	return a.Admin.EnableOwned(ctx, subject, op)
}

func (a *durableAdmin) SendVerification(ctx context.Context, subject, op string) error {
	if a.db.phase(ctx, a.issuer, a.key) != registration.EmailSending {
		panic(errSafe)
	}

	a.emails++
	return a.Admin.SendVerification(ctx, subject, op)
}

type durableProfiles struct {
	db       *durableLab
	lab      *lab
	group    string
	failOnce bool
}

func (p *durableProfiles) Persist(ctx context.Context, r registration.ProfileRecord) error {
	p.lab.verifyGroupIdentity(ctx, r.Subject, r.OperationID, p.group)
	if p.failOnce {
		p.failOnce = false
		return postgres.ErrStore
	}

	return p.db.store.Persist(ctx, r)
}

func (l *lab) durableService(d *durableLab, cfg keycloak.RegistrationConfig, group string, n int, failProfile bool) (*registration.Service, *durableAdmin, func()) {
	real, e := keycloak.NewRegistrationAdmin(cfg, l.client)
	if e != nil {
		panic(errSafe)
	}

	a := &durableAdmin{Admin: real, db: d, issuer: l.base + "/realms/" + l.realm, key: combinedKey(n)}
	service, e := registration.NewService(
		registration.ServiceConfig{
			Issuer:            a.issuer,
			FingerprintKey:    d.fingerprintKey,
			Timeout:           25 * time.Second,
			EmailRetryBackoff: time.Second,
			MaxEmailAttempts:  3,
		},
		a,
		&durableProfiles{db: d, lab: l, group: group, failOnce: failProfile},
		d.store,
	)
	if e != nil {
		real.Close()
		panic(errSafe)
	}

	return service, a, real.Close
}

// This is a private, trusted-TLS HTTP test endpoint; cmd/api is not modified or
// exposed. It exercises the existing real HTTP handler, not a fake controller.
func durableHTTP(ctx context.Context, service *registration.Service, key string, body io.Reader) (int, []byte, error) {
	previous := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(previous)
	limiter, e := handler.NewLocalRegistrationLimiter(
		handler.RegistrationLimitConfig{
			Key:           freshKey(),
			IPAttempts:    10,
			EmailAttempts: 10,
			IPWindow:      time.Minute,
			EmailWindow:   time.Minute,
			MaxKeys:       100,
			MaxConcurrent: 2,
		},
	)
	if e != nil {
		return 0, nil, errSafe
	}

	h, e := handler.NewRegistrationHandler(
		service,
		limiter,
		handler.RegistrationHTTPConfig{Timeout: 28 * time.Second, UnavailableRetryAfter: time.Second},
	)
	if e != nil {
		return 0, nil, errSafe
	}

	router := gin.New()
	router.POST("/api/v1/registrations", h.Handle)
	return durableRequest(ctx, router, key, body)
}

func durableRequest(ctx context.Context, router http.Handler, key string, body io.Reader) (int, []byte, error) {
	server := httptest.NewTLSServer(router)
	defer server.Close()
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/v1/registrations", body)
	if e != nil {
		return 0, nil, errSafe
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	client := server.Client()
	client.Timeout = 30 * time.Second
	defer client.CloseIdleConnections()
	response, e := client.Do(req)
	if e != nil {
		return 0, nil, errSafe
	}

	defer response.Body.Close()
	result, e := io.ReadAll(io.LimitReader(response.Body, 4097))
	if e != nil || len(result) > 4096 || response.Header.Get("Cache-Control") != "no-store" {
		return 0, nil, errSafe
	}

	return response.StatusCode, result, nil
}

func (l *lab) postgresScenario(ctx context.Context, caFile string) error {
	fmt.Fprintln(l.out, "POSTGRES_KEYCLOAK_STORES REAL_POSTGRESQL")
	fmt.Fprintln(l.out, "POSTGRES_KEYCLOAK_GROUP_AUTHORITY ACCEPTED_PROTOTYPE_LIMITATION")
	d, e := newDurableLab(ctx)
	if e != nil {
		fmt.Fprintln(l.out, "POSTGRES_SETUP BLOCKED")
		return errSafe
	}

	defer func() {
		if d.close() != nil {
			l.lifeCheck("POSTGRES_CLEANUP", false)
		} else {
			l.lifeCheck("POSTGRES_CLEANUP", true)
		}
	}()
	cfg, group, principal, rm := l.combinedSetup(ctx)
	if e = l.liveTLS(ctx); e != nil {
		return e
	}

	issuer := l.base + "/realms/" + l.realm
	service, a, closeAdmin := l.durableService(d, cfg, group, 501, false)
	defer closeAdmin()
	email := "durable" + randomID() + "@example.test"
	password := randomID()
	status, receipt, e := durableHTTP(ctx, service, a.key, combinedBody(501, email, password))
	if e != nil || status != http.StatusAccepted {
		fmt.Fprintln(l.out, "POSTGRES_HTTP_REGISTRATION FAIL")
		return errSafe
	}

	op := d.operation(ctx, issuer, a.key)
	user := l.realUser(ctx, op.Subject)
	credentials := l.realCredentials(ctx, op.Subject)
	l.lifeCheck(
		"POSTGRES_HTTP_REGISTRATION",
		op.Phase == registration.Completed &&
			op.EmailState == "request_accepted" &&
			user["enabled"] == true &&
			user["emailVerified"] == false &&
			containsAction(user, "VERIFY_EMAIL") &&
			d.profile(ctx, issuer, op.Subject, op.OperationID) &&
			a.creates == 1 &&
			a.enables == 1 &&
			a.emails == 1,
	)
	if _, e = l.verificationLink(ctx, email); e != nil {
		return errSafe
	}

	l.lifeCheck("POSTGRES_MAIL_DELIVERY", true)
	if d.reopen(ctx) != nil {
		return errSafe
	}

	replay, ra, closeReplay := l.durableService(d, cfg, group, 501, false)
	defer closeReplay()
	status, replayReceipt, e := durableHTTP(ctx, replay, ra.key, combinedBody(501, email, "changed-password-not-used"))
	reopened := d.operation(ctx, issuer, ra.key)
	l.lifeCheck(
		"POSTGRES_REOPEN_REPLAY",
		e == nil &&
			status == http.StatusAccepted &&
			bytes.Equal(receipt, replayReceipt) &&
			reopened.OperationID == op.OperationID &&
			reopened.Subject == op.Subject &&
			ra.creates == 0 &&
			ra.enables == 0 &&
			ra.emails == 0 &&
			reflect.DeepEqual(credentials, l.realCredentials(ctx, op.Subject)),
	)
	// Another key/email using the same NIC must not reach Keycloak.
	duplicate, da, closeDuplicate := l.durableService(d, cfg, group, 502, false)
	defer closeDuplicate()
	otherEmail := "notcreated" + randomID() + "@example.test"
	status, duplicateReceipt, e := durableHTTP(ctx, duplicate, da.key, combinedBody(501, otherEmail, randomID()))
	l.lifeCheck(
		"POSTGRES_NIC_DUPLICATE_NO_IDENTITY",
		e == nil &&
			status == http.StatusAccepted &&
			bytes.Equal(receipt, duplicateReceipt) &&
			da.creates == 0 &&
			l.userCount(ctx, otherEmail) == 0,
	)
	// Duplicate email under another NIC is rejected by live Keycloak, without adoption.
	ds, ea, closeEmailDuplicate := l.durableService(d, cfg, group, 503, false)
	defer closeEmailDuplicate()
	status, emailDuplicateReceipt, e := durableHTTP(ctx, ds, ea.key, combinedBody(503, email, randomID()))
	duplicateOp := d.operation(ctx, issuer, ea.key)
	l.lifeCheck(
		"POSTGRES_EMAIL_DUPLICATE_NO_ADOPTION",
		e == nil &&
			status == http.StatusAccepted &&
			bytes.Equal(receipt, emailDuplicateReceipt) &&
			duplicateOp.Phase == registration.Duplicate &&
			duplicateOp.Subject == "" &&
			ea.enables == 0 &&
			ea.emails == 0 &&
			l.userCount(ctx, email) == 1 &&
			reflect.DeepEqual(credentials, l.realCredentials(ctx, op.Subject)),
	)
	l.grant(ctx, principal, rm, "query-users")
	roles := decode[[]object](
		l.must(
			ctx,
			"GET",
			"/admin/realms/"+l.realm+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite",
			nil,
			200,
		),
	)
	l.lifeCheck("POSTGRES_SCOPED_ROLES", len(roles) == 1 && roles[0]["name"] == "query-users")
	ps, pa, closeProfile := l.durableService(d, cfg, group, 504, true)
	defer closeProfile()
	profileEmail := "profile" + randomID() + "@example.test"
	_, e = ps.Register(ctx, pa.key, combinedBody(504, profileEmail, randomID()))
	po := d.operation(ctx, issuer, pa.key)
	pc := l.realCredentials(ctx, po.Subject)
	l.lifeCheck(
		"POSTGRES_PROFILE_FAILURE_DISABLED",
		serviceCode(e, "registration_unavailable") &&
			po.Phase == registration.SubjectRecorded &&
			l.realUser(ctx, po.Subject)["enabled"] == false &&
			!d.profile(ctx, issuer, po.Subject, po.OperationID) &&
			pa.enables == 0,
	)
	if d.reopen(ctx) != nil {
		return errSafe
	}

	recovered, rpa, closeRecovery := l.durableService(d, cfg, group, 504, false)
	defer closeRecovery()
	_, e = recovered.Resume(ctx, rpa.key)
	po = d.operation(ctx, issuer, rpa.key)
	l.lifeCheck(
		"POSTGRES_PASSWORD_FREE_PROFILE_RECOVERY",
		e == nil &&
			po.Phase == registration.Completed &&
			rpa.creates == 0 &&
			rpa.enables == 1 &&
			d.profile(ctx, issuer, po.Subject, po.OperationID) &&
			reflect.DeepEqual(pc, l.realCredentials(ctx, po.Subject)),
	)
	as, aa, closeAmbiguous := l.durableService(d, cfg, group, 505, false)
	defer closeAmbiguous()
	aa.discardCreate = true
	ambiguousEmail := "ambiguous" + randomID() + "@example.test"
	_, e = as.Register(ctx, aa.key, combinedBody(505, ambiguousEmail, randomID()))
	ao := d.operation(ctx, issuer, aa.key)
	l.lifeCheck(
		"POSTGRES_CREATE_RESULT_DISCARDED",
		serviceCode(e, "registration_reconciliation_required") &&
			ao.Phase == registration.Creating &&
			ao.Subject == "" &&
			aa.creates == 1 &&
			l.userCount(ctx, ambiguousEmail) == 1,
	)
	if d.reopen(ctx) != nil {
		return errSafe
	}

	ars, ara, closeReconciled := l.durableService(d, cfg, group, 505, false)
	defer closeReconciled()
	_, e = ars.Resume(ctx, ara.key)
	ao = d.operation(ctx, issuer, ara.key)
	l.lifeCheck(
		"POSTGRES_MARKER_RECOVERY_AFTER_REOPEN",
		e == nil &&
			ao.Phase == registration.Completed &&
			ara.creates == 0 &&
			ara.finds == 1 &&
			ara.enables == 1 &&
			l.userCount(ctx, ambiguousEmail) == 1,
	)
	p := "/admin/realms/" + l.realm
	l.must(ctx, "PUT", p, object{"smtpServer": smtpSettings("1")}, 204)
	es, sa, closeSMTP := l.durableService(d, cfg, group, 506, false)
	defer closeSMTP()
	_, e = es.Register(ctx, sa.key, combinedBody(506, "smtp"+randomID()+"@example.test", randomID()))
	so := d.operation(ctx, issuer, sa.key)
	l.lifeCheck(
		"POSTGRES_SMTP_FAILURE_DURABLE_PENDING",
		serviceCode(e, "registration_reconciliation_required") &&
			so.Phase == registration.EmailSending &&
			d.profile(ctx, issuer, so.Subject, so.OperationID) &&
			sa.emails == 1 &&
			l.realUser(ctx, so.Subject)["enabled"] == true,
	)
	l.must(ctx, "PUT", p, object{"smtpServer": smtpSettings("1025")}, 204)
	if d.reopen(ctx) != nil {
		return errSafe
	}

	ers, era, closeSMTPRecovery := l.durableService(d, cfg, group, 506, false)
	defer closeSMTPRecovery()
	_, e = ers.Resume(ctx, era.key)
	l.lifeCheck(
		"POSTGRES_UNCERTAIN_EMAIL_NOT_RESENT",
		serviceCode(e, "registration_reconciliation_required") &&
			era.creates == 0 &&
			era.emails == 0 &&
			d.operation(ctx, issuer, era.key).Phase == registration.EmailSending,
	)
	// The real runtime constructor and router are exercised with trusted TLS;
	// this listener remains private to the disposable integration test.
	runtimeHandler, closeRuntime, e := wiring.Registration(
		config.Registration{
			Enabled:        true,
			Admin:          cfg,
			FingerprintKey: d.fingerprintKey,
			RateKey:        freshKey(),
			CAFile:         caFile,
		},
		d.store,
	)
	if e != nil {
		return errSafe
	}

	defer closeRuntime()
	previousMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(previousMode)
	router := handler.NewRouter(handler.RouterOptions{
		CORSOrigins:  []string{"https://web.example.test"},
		Registration: runtimeHandler,
	})
	runtimeEmail := "runtime" + randomID() + "@example.test"
	status, _, e = durableRequest(ctx, router, combinedKey(507), combinedBody(507, runtimeEmail, randomID()))
	if e != nil || status != http.StatusAccepted {
		l.lifeCheck("POSTGRES_RUNTIME_COMPOSITION", false)
		return errSafe
	}

	runtimeOp := d.operation(ctx, issuer, combinedKey(507))
	l.lifeCheck(
		"POSTGRES_RUNTIME_COMPOSITION",
		runtimeOp.Phase == registration.Completed &&
			d.profile(ctx, issuer, runtimeOp.Subject, runtimeOp.OperationID) &&
			l.realUser(ctx, runtimeOp.Subject)["enabled"] == true,
	)
	return nil
}
