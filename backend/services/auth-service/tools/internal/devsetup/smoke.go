package devsetup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/joho/godotenv"
)

// Synthetic input only. No password, token or action link is ever persisted.
type smokeCase struct {
	Key   string `json:"key"`
	NIC   string `json:"nic"`
	Email string `json:"email"`
}

func (a *localAdmin) smoke(ctx context.Context, root string) error {
	apiClient := *a.client
	apiClient.Timeout = 35 * time.Second
	if a.check(ctx) != nil {
		return ErrSetup
	}

	serviceRoot := filepath.Clean(filepath.Join(root, "..", "..", "backend", "services", "auth-service"))
	path := filepath.Join(serviceRoot, ".local", "registration-smoke.json")
	var sample smokeCase
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		hex := randomSecret()[:32]
		n, e := rand.Int(rand.Reader, big.NewInt(1_000_000_000_000))
		if e != nil {
			return ErrSetup
		}

		sample = smokeCase{
			Key:   hex[:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:],
			NIC:   fmt.Sprintf("%012d", n.Int64()),
			Email: "govai-local-smoke-" + randomSecret()[:16] + "@govai.test",
		}
		b, _ = json.Marshal(sample)
		if writeNew(path, b) != nil {
			return ErrSetup
		}
	} else if e != nil || json.Unmarshal(b, &sample) != nil {
		return ErrSetup
	}

	if sample.Key == "" || sample.Email == "" {
		return ErrSetup
	}

	ready, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://localhost:5001/api/ready", nil)
	if e != nil {
		return ErrSetup
	}

	response, e := apiClient.Do(ready)
	if e != nil {
		return ErrSetup
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	if response.StatusCode != 200 {
		return ErrSetup
	}

	fmt.Fprintln(a.out, "DEV_AUTH_API_TLS_READINESS PASS")
	body, _ := json.Marshal(
		object{
			"full_name":     "GovAI Synthetic Local Citizen",
			"nic":           sample.NIC,
			"mobile_number": "+14155552671",
			"email":         sample.Email,
			"password":      randomSecret(),
		},
	)
	req, e := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://localhost:5001/api/v1/registrations",
		bytes.NewReader(body),
	)
	if e != nil {
		return ErrSetup
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", sample.Key)
	// The shared client trusts our CA, never disables verification. Administrative
	// authorization is set per admin request only and is NOT sent to the Go API.
	result, e := apiClient.Do(req)
	if e != nil {
		return ErrSetup
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(result.Body, 4096))
	_ = result.Body.Close()
	if result.StatusCode != 202 {
		fmt.Fprintf(a.out, "DEV_AUTH_API_REGISTRATION status=%d BLOCKED\n", result.StatusCode)
		return ErrSetup
	}

	env, e := godotenv.Read(filepath.Join(serviceRoot, ".env.database"))
	if e != nil {
		return ErrSetup
	}

	u, e := url.Parse(env["DATABASE_URL"])
	if e != nil || u.Hostname() != "127.0.0.1" || u.Port() != "55432" {
		return ErrSetup
	}

	pool, e := postgres.Open(ctx, env["DATABASE_URL"])
	if e != nil {
		return ErrSetup
	}

	defer pool.Close()
	dataKey, e := base64.StdEncoding.DecodeString(env["PROFILE_DATA_KEY"])
	if e != nil {
		return ErrSetup
	}

	nicKey, e := base64.StdEncoding.DecodeString(env["NIC_INDEX_KEY"])
	if e != nil {
		return ErrSetup
	}

	store, e := postgres.NewStore(pool, dataKey, nicKey)
	if e != nil {
		return ErrSetup
	}

	session, e := store.Open(ctx, base+"/realms/GovAI", sample.Key)
	if e != nil {
		return ErrSetup
	}

	op := session.Snapshot()
	cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
	e = session.Close(cleanup)
	cancel()
	if e != nil {
		return ErrSetup
	}

	if op.Phase != registration.Completed || op.EmailState != "request_accepted" || op.Subject == "" {
		fmt.Fprintln(a.out, "DEV_AUTH_DURABLE_COMPLETION BLOCKED")
		return ErrSetup
	}

	var saved bool
	if pool.QueryRow(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM auth.citizen_profiles WHERE operation_id=$1 AND issuer=$2 AND subject=$3)",
		op.OperationID,
		op.Issuer,
		op.Subject,
	).Scan(&saved) != nil ||
		!saved {
		return ErrSetup
	}

	user := a.get(ctx, realmPath+"/users/"+op.Subject)
	if user["enabled"] != true ||
		user["email"] != sample.Email ||
		!arrayField(user["attributes"], "govai_registration_operation", op.OperationID) {
		return ErrSetup
	}

	roles := a.list(ctx, realmPath+"/users/"+op.Subject+"/role-mappings/realm/composite")
	citizen := false
	for _, role := range roles {
		if role["name"] == "citizen" {
			citizen = true
		}
	}

	if !citizen {
		return ErrSetup
	}

	fmt.Fprintln(a.out, "DEV_AUTH_REAL_REGISTRATION_PROFILE_CITIZEN PASS")
	mailClient := &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	delivered := false
	for attempt := 0; attempt < 10; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:8025/api/v1/messages", nil)
		r, e := mailClient.Do(req)
		if e == nil {
			b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			_ = r.Body.Close()
			var inbox struct {
				Messages []struct{ To []struct{ Address string } }
			}
			if r.StatusCode == 200 && json.Unmarshal(b, &inbox) == nil {
				for _, message := range inbox.Messages {
					for _, recipient := range message.To {
						if recipient.Address == sample.Email {
							delivered = true
						}
					}
				}
			}
		}

		if delivered {
			break
		}

		select {
		case <-ctx.Done():
			return ErrSetup
		case <-time.After(200 * time.Millisecond):
		}
	}

	if !delivered {
		fmt.Fprintln(a.out, "DEV_AUTH_MAILPIT_DELIVERY BLOCKED")
		return ErrSetup
	}

	fmt.Fprintln(a.out, "DEV_AUTH_MAILPIT_DELIVERY PASS; synthetic sample retained, no password stored")
	return nil
}
