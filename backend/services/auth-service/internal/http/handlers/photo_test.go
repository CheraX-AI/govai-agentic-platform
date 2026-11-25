package handler_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type photoObjects struct{ data map[string][]byte }

func (o *photoObjects) Put(_ context.Context, k string, b []byte, _ string) error {
	o.data[k] = b
	return nil
}
func (o *photoObjects) Get(_ context.Context, k string) ([]byte, error) {
	b, ok := o.data[k]
	if !ok {
		return nil, profile.ErrUnavailable
	}
	return b, nil
}
func (o *photoObjects) Delete(_ context.Context, k string) error { delete(o.data, k); return nil }
func (s *endpointProfiles) SetCitizenPhoto(_ context.Context, issuer, subject string, revision int64, key string) (profile.Details, error) {
	d, ok := s.records[subject]
	if !ok || issuer != s.issuer {
		return profile.Details{}, profile.ErrNotFound
	}
	if revision != d.Revision {
		return profile.Details{}, profile.ErrConflict
	}
	d.PhotoObjectKey = key
	d.Revision++
	s.records[subject] = d
	return d, nil
}
func TestPhotoHTTPAdapters(t *testing.T) {
	issuer, verifier, sign := signedProfileTokens(t)
	store := &endpointProfiles{issuer: issuer, records: map[string]profile.Details{"citizen-a": {}, "citizen-b": {}}}
	profileService, _ := profile.NewService(store, profile.CitizenFieldRules{})
	objects := &photoObjects{data: map[string][]byte{}}
	photos, _ := profile.NewPhotoService(store, objects)
	native, _ := handler.NewProtected(verifier, store)
	_ = native.EnableProfile(profileService)
	_ = native.EnablePhoto(photos)
	session := &profileBrowser{caller: profile.Caller{Issuer: issuer, Subject: "citizen-a"}}
	limit, _ := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: make([]byte, 32), IPAttempts: 100, EmailAttempts: 1, IPWindow: time.Minute, EmailWindow: time.Minute, MaxKeys: 10, MaxConcurrent: 4})
	web, _ := handler.NewWebSessionHandler(session, limit, "https://govai.test")
	_ = web.EnableProfile(profileService)
	_ = web.EnablePhoto(photos)
	router := handler.NewRouter(handler.RouterOptions{Protected: native, WebSessions: web, CORSOrigins: []string{"https://govai.test"}})
	var imageBytes bytes.Buffer
	_ = jpeg.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil)
	exact := append(append([]byte{}, imageBytes.Bytes()...), make([]byte, profile.MaxPhotoBytes-imageBytes.Len())...)
	request := func(path, method, token, revision, csrf string, data []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Content-Type", "image/jpeg")
		if revision != "" {
			r.Header.Set("If-Match", revision)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		} else if strings.HasPrefix(path, "/api/v1/web/") {
			r.Header.Set("X-GovAI-Web", "1")
			r.Header.Set("Origin", "https://govai.test")
			r.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: syntheticWebSecret})
			if csrf != "" {
				r.Header.Set("X-CSRF-Token", csrf)
			}
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	token := sign(nil)
	path := "/api/v1/profile/photo"
	if request(path, "PUT", "", `"0"`, "", exact).Code != 401 {
		t.Fatal("unauthenticated upload accepted")
	}
	if request(path, "PUT", token, "", "", exact).Code != 428 {
		t.Fatal("missing revision accepted")
	}
	if w := request(path, "PUT", token, `"0"`, "", exact); w.Code != 200 {
		t.Fatal("2 MiB upload rejected", w.Code, w.Body.String())
	}
	if request(path, "PUT", token, `"0"`, "", imageBytes.Bytes()).Code != 409 {
		t.Fatal("stale replacement accepted")
	}
	if request(path, "GET", sign(map[string]any{"sub": "citizen-b"}), "", "", nil).Code != 404 {
		t.Fatal("citizen B read citizen A photo")
	}
	if w := request(path, "GET", token, "", "", nil); w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("photo retrieval failed")
	}
	if request(path, "PUT", token, `"1"`, "", make([]byte, 10<<20)).Code != 413 {
		t.Fatal("10 MiB upload accepted")
	}
	if request(path, "PUT", token, `"1"`, "", []byte("fake image")).Code != 415 {
		t.Fatal("forged image accepted")
	}
	if request("/api/v1/web/profile/photo", "PUT", "", `"1"`, "", exact).Code != 403 {
		t.Fatal("photo CSRF bypass")
	}
	if w := request("/api/v1/web/profile/photo", "PUT", "", `"1"`, syntheticWebSecret, exact); w.Code != 200 {
		t.Fatal("browser photo upload failed", w.Code, w.Body.String())
	}
	if request("/api/v1/web/profile/photo", "GET", "", "", "", nil).Code != 200 {
		t.Fatal("browser photo read failed")
	}
	for _, p := range []string{path, "/api/v1/web/profile/photo"} {
		r := httptest.NewRequest("OPTIONS", p, nil)
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Origin", "https://govai.test")
		r.Header.Set("Access-Control-Request-Method", "PUT")
		headers := "authorization,content-type,if-match"
		if strings.Contains(p, "/web/") {
			headers = "x-govai-web,x-csrf-token,content-type,if-match"
		}
		r.Header.Set("Access-Control-Request-Headers", headers)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatal("photo preflight failed")
		}
	}
}
