package chatgateway

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientBoundaries(t *testing.T) {
	for _, mode := range []string{
		"ok",
		"redirect",
		"denied",
		"failure",
		"oversized",
		"malformed",
		"media",
		"canceled",
		"untrusted",
	} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/chat/message" ||
					r.Method != "POST" ||
					r.Header.Get("Authorization") != "Bearer synthetic-access" ||
					r.Header.Get("Cookie") != "" {
					t.Error("invalid upstream boundary")
				}

				switch mode {
				case "redirect":
					w.Header().Set("Location", "/elsewhere")
					w.WriteHeader(302)
					return
				case "denied":
					w.WriteHeader(401)
					return
				case "failure":
					w.WriteHeader(503)
					return
				case "media":
					w.Header().Set("Content-Type", "text/html")
				default:
					w.Header().Set("Content-Type", "application/json")
				}

				if mode == "oversized" {
					fmt.Fprint(w, strings.Repeat("x", (64<<10)+1))
					return
				}

				if mode == "malformed" {
					fmt.Fprint(w, "private invalid body")
					return
				}

				fmt.Fprint(w, `{"answer":"Synthetic","sources":[]}`)
			}))
			defer server.Close()
			roots := x509.NewCertPool()
			if mode != "untrusted" {
				roots.AddCert(server.Certificate())
			}

			client, err := New(server.URL+"/api/chat/message", roots, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			defer client.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}

			result, err := client.Ask(ctx, "synthetic-access", []byte(`{"message":"Synthetic question"}`))
			if mode == "ok" {
				if err != nil || len(result.Bytes()) == 0 {
					t.Fatal("success failed")
				}

				if fmt.Sprint(result) != "[REDACTED]" {
					t.Fatal("payload exposed")
				}

				return
			}

			if err == nil {
				t.Fatal("unsafe response accepted")
			}

			if mode == "redirect" && calls != 1 {
				t.Fatal("redirect followed")
			}

			if mode == "denied" && !errors.Is(err, ErrDenied) {
				t.Fatal("wrong classification")
			}
		})
	}
}

func TestUnsafeEndpoints(t *testing.T) {
	for _, u := range []string{
		"http://localhost:5002/api/chat/message",
		"https://user:password@localhost/api/chat/message",
		"https://localhost/api/chat/message?target=other",
		"https://localhost/other",
	} {
		if _, err := New(u, nil, time.Second); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
