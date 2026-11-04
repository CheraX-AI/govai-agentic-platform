package oidccheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	for _, mode := range []string{"ok", "mismatch", "missing_uri", "external_uri", "redirect", "bad_json", "large", "http_error", "empty_keys", "bad_key", "keys_error", "keys_json", "keys_large", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var base string
			var keyCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("credentials sent")
				}
				if r.URL.Path == "/realms/GovAI/.well-known/openid-configuration" {
					switch mode {
					case "redirect":
						http.Redirect(w, r, "/keys", http.StatusFound)
						return
					case "bad_json":
						_, _ = w.Write([]byte("private upstream error"))
						return
					case "large":
						_, _ = w.Write([]byte(strings.Repeat("x", maxDocument+1)))
						return
					case "http_error":
						http.Error(w, "private upstream error", 503)
						return
					case "timeout":
						<-r.Context().Done()
						return
					}
					issuer, uri := base+"/realms/GovAI", base+"/keys"
					switch mode {
					case "mismatch":
						issuer += "wrong"
					case "missing_uri":
						uri = ""
					case "external_uri":
						uri = "http://localhost:1/keys"
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "jwks_uri": uri})
					return
				}
				if r.URL.Path != "/keys" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				keyCalls.Add(1)
				switch mode {
				case "keys_error":
					http.Error(w, "private", 500)
				case "keys_json":
					_, _ = w.Write([]byte("private"))
				case "keys_large":
					_, _ = w.Write([]byte(strings.Repeat("x", maxDocument+1)))
				case "empty_keys":
					_, _ = w.Write([]byte(`{"keys":[]}`))
				case "bad_key":
					_, _ = w.Write([]byte(`{"keys":[{}]}`))
				default:
					_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA"}]}`))
				}
			}))
			defer server.Close()
			base = server.URL
			timeout := time.Second
			if mode == "timeout" {
				timeout = 50 * time.Millisecond
			}
			err := Check(context.Background(), base+"/realms/GovAI", timeout)
			if (err == nil) != (mode == "ok") {
				t.Fatalf("mode %s error %v", mode, err)
			}
			if err != nil && (strings.Contains(err.Error(), base) || strings.Contains(err.Error(), "private")) {
				t.Fatal("unsafe error")
			}
			if mode == "mismatch" && keyCalls.Load() != 0 {
				t.Fatal("JWKS requested after mismatch")
			}
		})
	}
}

func TestValidationAndCancellation(t *testing.T) {
	for _, issuer := range []string{"", "http://example.com/realm", "https://user:private@example.com", "https://example.com?private=1", "https://example.com#", "/relative"} {
		if Check(context.Background(), issuer, time.Second) == nil {
			t.Fatal("invalid issuer accepted")
		}
	}
	for _, timeout := range []time.Duration{0, -time.Second, 31 * time.Second} {
		if Check(context.Background(), "http://localhost:8180/realms/GovAI", timeout) == nil {
			t.Fatal("invalid timeout")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("canceled request reached server") }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Check(ctx, server.URL, time.Second) == nil {
		t.Fatal("canceled check succeeded")
	}
	server.Close()
	if Check(context.Background(), server.URL, time.Second) == nil {
		t.Fatal("offline check succeeded")
	}
}
