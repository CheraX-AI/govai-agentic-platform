package websession

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
)

type fencedDependencies struct {
	*dependencies
	events   []string
	failMark bool
}

func (d *fencedDependencies) AuthenticationMark(context.Context) (int64, error) {
	d.events = append(d.events, "mark")
	if d.failMark {
		return 0, errors.New("private SQL")
	}

	return 42, nil
}

func (d *fencedDependencies) Login(ctx context.Context, r login.Request) (login.Access, error) {
	d.events = append(d.events, "login")
	return d.dependencies.Login(ctx, r)
}

func (d *fencedDependencies) Create(context.Context, postgres.WebCredentials, time.Time) (string, error) {
	return "", errors.New("unfenced creation forbidden")
}

func (d *fencedDependencies) CreateFenced(ctx context.Context, c postgres.WebCredentials, expiry time.Time, mark int64) (string, error) {
	d.events = append(d.events, "create_fenced")
	if mark != 42 {
		return "", errors.New("wrong mark")
	}

	return d.dependencies.Create(ctx, c, expiry)
}

func TestWebLoginCapturesFenceBeforeProvider(t *testing.T) {
	for _, fail := range []bool{false, true} {
		d := &fencedDependencies{dependencies: &dependencies{}, failMark: fail}
		service, err := New(d, d, d, d, d, time.Hour)
		if err != nil {
			t.Fatal(err)
		}

		request, _ := login.Parse(strings.NewReader(`{"email":"synthetic@example.test","password":"synthetic-password"}`))
		started, err := service.Start(context.Background(), request)
		if fail {
			if err == nil || !reflect.DeepEqual(d.events, []string{"mark"}) {
				t.Fatal("provider used without fence")
			}
		} else {
			if err != nil ||
				started.Cookie == "" ||
				!reflect.DeepEqual(d.events, []string{"mark", "login", "create_fenced"}) {
				t.Fatal("wrong guarded login order")
			}
		}
	}
}
