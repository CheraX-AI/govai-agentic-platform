package permissionlab

import (
	"context"
	"net/url"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

// baselinePayload freezes the original seven-field group-free permission probe.
// It is isolated experiment code, not a production configuration escape hatch.
func baselinePayload(r registration.Request, op string) object {
	return object{
		"username":        r.Email,
		"email":           r.Email,
		"enabled":         false,
		"emailVerified":   false,
		"requiredActions": []string{"VERIFY_EMAIL"},
		"attributes":      object{"govai_registration_operation": []string{op}},
		"credentials":     []object{{"type": "password", "value": r.TransientPassword(), "temporary": false}},
	}
}

func (l *lab) baselineCreate(ctx context.Context, token string, r registration.Request, op string) (registration.Identity, error) {
	p := "/admin/realms/" + l.realm + "/users"
	status, _, err := l.request(ctx, token, "POST", p, baselinePayload(r, op))
	if err != nil || status != 201 {
		return registration.Identity{}, errSafe
	}

	rows := decode[[]object](
		l.must(
			ctx,
			"GET",
			p+"?q="+url.QueryEscape("govai_registration_operation:"+op)+"&max=2",
			nil,
			200,
		),
	)
	if len(rows) != 1 {
		return registration.Identity{}, errSafe
	}

	attrs, ok := rows[0]["attributes"].(map[string]any)
	if !ok {
		return registration.Identity{}, errSafe
	}

	values, ok := attrs["govai_registration_operation"].([]any)
	if !ok || len(values) != 1 || values[0] != op {
		return registration.Identity{}, errSafe
	}

	subject, ok := rows[0]["id"].(string)
	if !ok || subject == "" {
		return registration.Identity{}, errSafe
	}

	return registration.Identity{Subject: subject}, nil
}
