package chat

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
)

func question(t *testing.T) chatcontract.Question {
	t.Helper()
	q, err := chatcontract.Parse(strings.NewReader(`{"message":"தமிழ் synthetic question"}`))
	if err != nil {
		t.Fatal(err)
	}

	return q
}

func citizen() context.Context {
	return WithCitizen(context.Background(), "synthetic-subject")
}

func TestHTTPAgentPayloadAndServiceAuthentication(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "POST" ||
			r.URL.Path != "/chat" ||
			r.URL.RawQuery != "" ||
			r.Header.Get("Authorization") != "Bearer synthetic-service-secret" ||
			r.Header.Get("X-GovAI-Citizen") != "synthetic-subject" ||
			r.Header.Get("Cookie") != "" ||
			r.Header.Get("Accept") != "application/json" {
			t.Error("incorrect agent request")
		}

		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil ||
			len(body) != 1 ||
			body["message"] != "தமிழ் synthetic question" {
			t.Error("incorrect question envelope")
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(
			w,
			`{"answer":"Synthetic response.","sources":[{"number":1,"title":"Synthetic source","url":"https://official.example.test/doc?lang=ta#section"}]}`,
		)
	}))
	defer server.Close()
	cfg := HTTPConfig{
		URL:         server.URL + "/chat",
		AuthMode:    "bearer",
		BearerToken: "synthetic-service-secret",
		Timeout:     time.Second,
	}
	agent, err := NewHTTP(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	defer agent.Close()
	reply, err := agent.Ask(citizen(), question(t))
	if err != nil ||
		reply.Answer != "Synthetic response." ||
		len(reply.Sources) != 1 ||
		reply.Sources[0].URL != "https://official.example.test/doc?lang=ta#section" ||
		requests.Load() != 1 {
		t.Fatal("transport failed or changed response")
	}

	encoded, _ := json.Marshal(agent)
	if strings.Contains(string(encoded)+fmt.Sprintf("%+v %#v %+v", agent, agent, cfg), cfg.BearerToken) {
		t.Fatal("service secret disclosure")
	}
}

func TestHTTPAgentRejectsUnsafeConfiguration(t *testing.T) {
	for _, cfg := range []HTTPConfig{
		{URL: "http://127.0.0.1:8002/chat", AuthMode: "none", Timeout: time.Second},
		{
			URL:               "http://agent.example.test/chat",
			AuthMode:          "none",
			AllowLoopbackHTTP: true,
			Timeout:           time.Second,
		},
		{
			URL:               "http://localhost:8002/chat",
			AuthMode:          "none",
			AllowLoopbackHTTP: true,
			Timeout:           time.Second,
		},
		{URL: "https://agent.example.test/chat", AuthMode: "none", Timeout: time.Second},
		{
			URL:         "https://user:pass@agent.example.test/chat",
			AuthMode:    "bearer",
			BearerToken: "synthetic",
			Timeout:     time.Second,
		},
		{
			URL:         "https://agent.example.test/chat?url=caller",
			AuthMode:    "bearer",
			BearerToken: "synthetic",
			Timeout:     time.Second,
		},
		{
			URL:         "https://agent.example.test/chat#fragment",
			AuthMode:    "bearer",
			BearerToken: "synthetic",
			Timeout:     time.Second,
		},
		{
			URL:         "https://agent.example.test/a/../chat",
			AuthMode:    "bearer",
			BearerToken: "synthetic",
			Timeout:     time.Second,
		},
		{
			URL:         "https://agent.example.test/chat",
			AuthMode:    "bearer",
			BearerToken: "bad\r\nheader",
			Timeout:     time.Second,
		},
	} {
		if _, err := NewHTTP(cfg, &http.Client{}); err == nil {
			t.Fatal("unsafe config accepted")
		}
	}

	cfg := HTTPConfig{URL: "https://127.0.0.1/chat", AuthMode: "none", Timeout: time.Second}
	if _, err := NewHTTP(
		cfg,
		&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}},
	); err == nil {
		t.Fatal("TLS bypass accepted")
	}
}

func TestHTTPAgentDoesNotFollowRedirectOrRetry(t *testing.T) {
	var forwarded, requests atomic.Int32
	target := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			forwarded.Add(1)
			w.WriteHeader(200)
		}),
	)
	defer target.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Location", target.URL+"/capture")
		w.WriteHeader(307)
	}))
	defer server.Close()
	agent, err := NewHTTP(
		HTTPConfig{
			URL:         server.URL + "/chat",
			AuthMode:    "bearer",
			BearerToken: "synthetic-service-secret",
			Timeout:     time.Second,
		},
		server.Client(),
	)
	if err != nil {
		t.Fatal(err)
	}

	defer agent.Close()
	if _, err = agent.Ask(citizen(), question(t)); !errors.Is(err, ErrResponse) ||
		requests.Load() != 1 ||
		forwarded.Load() != 0 {
		t.Fatal("redirect or retry occurred")
	}
}

func TestHTTPAgentTimeoutCancellationAndTLS(t *testing.T) {
	server := httptest.NewTLSServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {

				_, _ = io.Copy(io.Discard, r.Body)
				<-r.Context().Done()
			},
		),
	)
	defer server.Close()
	cfg := HTTPConfig{URL: server.URL + "/chat", AuthMode: "none", Timeout: 50 * time.Millisecond}
	agent, err := NewHTTP(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	defer agent.Close()
	if _, err = agent.Ask(citizen(), question(t)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("timeout not enforced")
	}

	ctx, cancel := context.WithCancel(citizen())
	cancel()
	if _, err = agent.Ask(ctx, question(t)); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation not propagated")
	}

	untrusted, err := NewHTTP(cfg, &http.Client{})
	if err != nil {
		t.Fatal(err)
	}

	defer untrusted.Close()
	if _, err = untrusted.Ask(citizen(), question(t)); err == nil {
		t.Fatal("untrusted certificate accepted")
	}
}

func TestHTTPAgentResponseFailures(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		media, body string
		want        error
	}{
		{"status", 500, "application/json", `{"error":"private backend details"}`, ErrUnavailable},
		{"rate", 429, "application/json", `{}`, ErrUnavailable},
		{"auth", 401, "application/json", `{}`, ErrResponse},
		{"busy", 409, "application/json", `{"detail":"private"}`, ErrBusy},
		{"no conversation named", 404, "application/json", `{}`, ErrResponse},
		{"media", 200, "text/html", `<p>private</p>`, ErrResponse},
		{"invalid", 200, "application/json", `{"answer":`, ErrResponse},
		{"empty", 200, "application/json", `{"answer":" "}`, ErrResponse},
		{"duplicate", 200, "application/json", `{"answer":"a","answer":"b"}`, ErrResponse},
		{"overflow", 200, "application/json", strings.Repeat("x", chatcontract.MaxResponseBytes+1), ErrResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var count atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				w.Header().Set("Content-Type", tc.media)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			agent, err := NewHTTP(
				HTTPConfig{URL: server.URL + "/chat", AuthMode: "none", Timeout: time.Second},
				server.Client(),
			)
			if err != nil {
				t.Fatal(err)
			}

			defer agent.Close()
			if _, err = agent.Ask(citizen(), question(t)); !errors.Is(err, tc.want) ||
				count.Load() != 1 ||
				strings.Contains(err.Error(), "private") {
				t.Fatal("response failure not safely handled")
			}
		})
	}
}

func TestHTTPAgentForwardsTheConversationAndMapsNotFound(t *testing.T) {
	var sent atomic.Value
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		sent.Store(body["conversation_id"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"detail":"Conversation not found."}`)
	}))
	defer server.Close()
	agent, err := NewHTTP(HTTPConfig{URL: server.URL + "/chat", AuthMode: "none", Timeout: time.Second}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	defer agent.Close()
	q, err := chatcontract.Parse(strings.NewReader(`{"message":"synthetic","conversation_id":"3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"}`))
	if err != nil {
		t.Fatal(err)
	}

	if _, err = agent.Ask(citizen(), q); !errors.Is(err, ErrNotFound) || sent.Load() != "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34" {
		t.Fatal("conversation not forwarded or 404 not mapped")
	}
}

func TestHTTPAgentNeverCallsWithoutACitizen(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer server.Close()
	agent, err := NewHTTP(HTTPConfig{URL: server.URL + "/chat", AuthMode: "none", Timeout: time.Second}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	defer agent.Close()
	for _, ctx := range []context.Context{
		context.Background(),
		WithCitizen(context.Background(), ""),
		WithCitizen(context.Background(), "has space"),
		WithCitizen(context.Background(), strings.Repeat("x", 256)),
	} {
		if _, err = agent.Ask(ctx, question(t)); !errors.Is(err, ErrUnavailable) {
			t.Fatal("agent called without a valid citizen")
		}
	}

	if requests.Load() != 0 {
		t.Fatal("request sent without a citizen")
	}
}
