package handler_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/objectstorage"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func profileFlowID(t *testing.T) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal("test identifier generation failed")
	}
	return hex.EncodeToString(value[:])
}

// Same opt-in and loopback restriction as the existing PostgreSQL tests. Only
// the randomly created database is migrated/dropped; never the configured DB.
func profileFlowDatabase(t *testing.T) (*postgres.Store, *postgres.Store, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("GOVAI_DB_TEST") != "1" {
		t.Skip("opt-in disposable PostgreSQL integration: GOVAI_DB_TEST=1")
	}
	ownerEnv, err := godotenv.Read("../../../.env.migrate")
	if err != nil {
		t.Fatal("disposable migration settings required")
	}
	appEnv, err := godotenv.Read("../../../.env")
	if err != nil {
		t.Fatal("disposable application settings required")
	}
	ownerURL, err := url.Parse(ownerEnv["MIGRATION_DATABASE_URL"])
	if err != nil || ownerURL.Hostname() != "127.0.0.1" || ownerURL.Port() != "55432" {
		t.Fatal("test database restricted to isolated loopback cluster")
	}
	appURL, err := url.Parse(appEnv["DATABASE_URL"])
	if err != nil || appURL.Host != ownerURL.Host || appURL.User == nil || appURL.User.Username() != "govai_auth_app" {
		t.Fatal("restricted test application role required")
	}
	ctx := context.Background()
	owner, err := postgres.Open(ctx, ownerURL.String())
	if err != nil {
		t.Fatal("test cluster unavailable")
	}
	name := "govai_auth_test_" + profileFlowID(t)
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := owner.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		owner.Close()
		t.Fatal("test database creation failed")
	}
	t.Cleanup(func() {
		defer owner.Close()
		if !strings.HasPrefix(name, "govai_auth_test_") || len(name) != 48 {
			t.Error("invalid cleanup target")
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := owner.Exec(cleanup, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error("test database cleanup failed")
		}
	})
	ownerURL.Path = "/" + name
	admin, err := postgres.Open(ctx, ownerURL.String())
	if err != nil {
		t.Fatal("test migration pool unavailable")
	}
	t.Cleanup(admin.Close)
	if postgres.Migrate(ctx, admin) != nil || postgres.MigrateCitizenDetails(ctx, admin) != nil {
		t.Fatal("test migrations failed")
	}
	if _, err := admin.Exec(ctx, "REVOKE ALL ON DATABASE "+quoted+" FROM PUBLIC; GRANT CONNECT ON DATABASE "+quoted+" TO govai_auth_app; REVOKE ALL ON SCHEMA public FROM PUBLIC"); err != nil {
		t.Fatal("test database grants failed")
	}
	appURL.Path = "/" + name
	open := func() *postgres.Store {
		pool, err := postgres.Open(ctx, appURL.String())
		if err != nil {
			t.Fatal("test application pool unavailable")
		}
		t.Cleanup(pool.Close)
		store, err := postgres.NewStore(pool, bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
		if err != nil || postgres.CitizenDetailsReady(ctx, pool) != nil {
			t.Fatal("restricted profile store not ready")
		}
		return store
	}
	return open(), open(), admin
}

func profileFlowCitizen(t *testing.T, store *postgres.Store, issuer, subject, nicText string) {
	t.Helper()
	ctx := context.Background()
	nic, err := registration.ParseNIC(nicText)
	if err != nil {
		t.Fatal("invalid synthetic NIC")
	}
	id := profileFlowID(t)
	key := id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
	operation, err := store.Reserve(ctx, registration.Reservation{Issuer: issuer, Key: key, OperationID: profileFlowID(t), Fingerprint: [32]byte{9}, Pending: registration.ProfileData{FullName: "Synthetic " + subject, NIC: nic, MobileNumber: "+94770000000"}})
	if err != nil {
		t.Fatal("synthetic registration reserve failed")
	}
	defer func() {
		if operation.Close(ctx) != nil {
			t.Error("registration session close failed")
		}
	}()
	advance := func(phase registration.Phase) {
		next := operation.Snapshot()
		next.Revision++
		next.Phase = phase
		if phase == registration.SubjectRecorded {
			next.Subject = subject
		}
		if operation.Save(ctx, next) != nil {
			t.Fatal("synthetic registration transition failed")
		}
	}
	advance(registration.Creating)
	advance(registration.SubjectRecorded)
	snapshot := operation.Snapshot()
	if store.Persist(ctx, registration.ProfileRecord{Issuer: issuer, Subject: subject, OperationID: snapshot.OperationID, Data: snapshot.Pending}) != nil {
		t.Fatal("linked registration profile persistence failed")
	}
	advance(registration.ProfileReady)
	advance(registration.Enabling)
	advance(registration.Enabled)
}

func TestProfileCrossAdapterDatabaseFlow(t *testing.T) {
	store, freshStore, admin := profileFlowDatabase(t)
	issuer, verifier, sign := signedProfileTokens(t)
	profileFlowCitizen(t, store, issuer, "citizen-a", "123456789v")
	profileFlowCitizen(t, store, issuer, "citizen-b", "223456789v")

	// Real S3 adapter, synthetic signed HTTP storage. This is not SeaweedFS.
	objects := map[string][]byte{}
	var storageMu sync.Mutex
	failUpload := false
	storageHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageMu.Lock()
		defer storageMu.Unlock()
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("storage request not signed")
		}
		switch r.Method {
		case http.MethodPut:
			if failUpload {
				w.WriteHeader(503)
				return
			}
			objects[r.URL.Path], _ = io.ReadAll(io.LimitReader(r.Body, profile.MaxPhotoBytes+1))
		case http.MethodGet:
			data, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(404)
				return
			}
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
		default:
			w.WriteHeader(405)
		}
	}))
	t.Cleanup(storageHTTP.Close)
	storage, err := objectstorage.New(objectstorage.Config{Endpoint: storageHTTP.URL, Bucket: "profile-flow-test", Region: "test-region", AccessKey: "synthetic-key", SecretKey: "synthetic-secret"})
	if err != nil {
		t.Fatal("test storage adapter unavailable")
	}
	t.Cleanup(storage.Close)
	service, _ := profile.NewService(store, profile.CitizenFieldRules{})
	photos, _ := profile.NewPhotoService(store, storage)
	native, _ := handler.NewProtected(verifier, store)
	if native.EnableProfile(service) != nil || native.EnablePhoto(photos) != nil {
		t.Fatal("native adapter setup failed")
	}
	identity := &profileBrowser{caller: profile.Caller{Issuer: issuer, Subject: "citizen-a", Email: "synthetic@example.test"}}
	limit, _ := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: make([]byte, 32), IPAttempts: 100, EmailAttempts: 1, IPWindow: time.Minute, EmailWindow: time.Minute, MaxKeys: 10, MaxConcurrent: 4})
	web, _ := handler.NewWebSessionHandler(identity, limit, "https://govai.test")
	if web.EnableProfile(service) != nil || web.EnablePhoto(photos) != nil {
		t.Fatal("browser adapter setup failed")
	}
	var logs bytes.Buffer
	router := handler.NewRouter(handler.RouterOptions{Protected: native, WebSessions: web, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	request := func(browser bool, method, path, subject, revision, media string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Content-Type", media)
		if revision != "" {
			r.Header.Set("If-Match", revision)
		}
		if browser {
			identity.caller.Subject = subject
			r.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: syntheticWebSecret})
			r.Header.Set("X-GovAI-Web", "1")
			r.Header.Set("Origin", "https://govai.test")
			r.Header.Set("X-CSRF-Token", syntheticWebSecret)
		} else {
			r.Header.Set("Authorization", "Bearer "+sign(map[string]any{"sub": subject}))
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	read := func(browser bool, subject string) map[string]any {
		path := "/api/v1/profile"
		if browser {
			path = "/api/v1/web/profile"
		}
		w := request(browser, "GET", path, subject, "", "application/json", nil)
		var value map[string]any
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("profile read failed or cached", w.Code)
		}
		return value
	}
	if read(true, "citizen-a")["revision"] != float64(0) {
		t.Fatal("registration details row not initialized")
	}
	fields := map[string]any{"revision": 0, "name_si": "සිංහල පරීක්ෂණ නම", "name_ta": "தமிழ் சோதனை பெயர்", "date_of_birth": "", "gender": "female", "address": map[string]string{"line1": "Private synthetic road", "line2": "Unit 4", "city": "Synthetic city"}}
	body, _ := json.Marshal(fields)
	if request(true, "PUT", "/api/v1/web/profile", "citizen-a", "", "application/json", body).Code != 200 {
		t.Fatal("browser edit failed")
	}
	if got := read(false, "citizen-a"); got["revision"] != float64(1) || got["name_ta"] != fields["name_ta"] {
		t.Fatal("native client did not see browser edit")
	}
	fields["revision"] = 1
	fields["address"].(map[string]string)["city"] = "Native synthetic city"
	body, _ = json.Marshal(fields)
	if request(false, "PUT", "/api/v1/profile", "citizen-a", "", "application/json", body).Code != 200 || read(true, "citizen-a")["revision"] != float64(2) {
		t.Fatal("native edit not visible on browser")
	}
	if request(true, "PUT", "/api/v1/web/profile", "citizen-a", "", "application/json", body).Code != 409 {
		t.Fatal("cross-client stale edit accepted")
	}
	fields["revision"] = 2
	fields["nic"] = "forged"
	forged, _ := json.Marshal(fields)
	if request(false, "PUT", "/api/v1/profile", "citizen-a", "", "application/json", forged).Code != 400 {
		t.Fatal("read-only identity edit accepted")
	}
	delete(fields, "nic")
	var jpegBytes bytes.Buffer
	if jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil) != nil {
		t.Fatal("test image creation failed")
	}
	if request(false, "PUT", "/api/v1/profile/photo", "citizen-a", `"2"`, "image/jpeg", jpegBytes.Bytes()).Code != 200 {
		t.Fatal("native photo upload failed")
	}
	photo := request(true, "GET", "/api/v1/web/profile/photo", "citizen-a", "", "", nil)
	if photo.Code != 200 || photo.Header().Get("Content-Type") != "image/jpeg" || read(true, "citizen-a")["revision"] != float64(3) {
		t.Fatal("browser did not see native photo")
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(photo.Body.Bytes())); err != nil {
		t.Fatal("stored photo not a valid image")
	}
	if request(false, "GET", "/api/v1/profile/photo", "citizen-b", "", "", nil).Code != 404 || read(false, "citizen-b")["revision"] != float64(0) {
		t.Fatal("citizen B accessed or inherited A's profile/photo")
	}
	if request(false, "PUT", "/api/v1/profile/photo?subject=citizen-a", "citizen-b", `"0"`, "image/jpeg", jpegBytes.Bytes()).Code != 400 {
		t.Fatal("caller-selected photo owner accepted")
	}
	before, _ := store.CitizenDetails(context.Background(), issuer, "citizen-a")
	storageMu.Lock()
	failUpload = true
	storageMu.Unlock()
	if request(false, "PUT", "/api/v1/profile/photo", "citizen-a", `"3"`, "image/jpeg", jpegBytes.Bytes()).Code != 503 {
		t.Fatal("storage failure not reported")
	}
	after, _ := freshStore.CitizenDetails(context.Background(), issuer, "citizen-a")
	storageMu.Lock()
	objectCount := len(objects)
	storageMu.Unlock()
	if after.Revision != before.Revision || after.PhotoObjectKey != before.PhotoObjectKey || objectCount != 1 || after.Fields.NameSI != fields["name_si"] {
		t.Fatal("failed upload broke persisted details/photo")
	}
	var ciphertext []byte
	if admin.QueryRow(context.Background(), "SELECT details_ciphertext FROM auth.citizen_details WHERE issuer=$1 AND subject=$2", issuer, "citizen-a").Scan(&ciphertext) != nil {
		t.Fatal("encrypted persisted details missing")
	}
	for _, secret := range []string{fields["name_si"].(string), fields["name_ta"].(string), "Private synthetic road", "Native synthetic city", "synthetic@example.test"} {
		if bytes.Contains(ciphertext, []byte(secret)) || strings.Contains(logs.String(), secret) {
			t.Fatal("personal data found in ciphertext/logs")
		}
	}
}
