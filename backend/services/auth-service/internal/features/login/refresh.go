package login

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SessionTokens must never be logged or serialized implicitly. HTTP adapters
// must explicitly select fields appropriate for native or server-side custody.
type SessionTokens struct {
	privateValue
	AccessToken, RefreshToken   string
	ExpiresIn, RefreshExpiresIn int64
}

func validRefresh(raw string) bool {
	if len(raw) == 0 || len(raw) > 16<<10 {
		return false
	}

	for _, c := range raw {
		if c < 33 || c > 126 {
			return false
		}
	}

	return true
}

// Refresh performs exactly one credential POST. An ambiguous network failure
// must not be retried automatically: the provider may have rotated the token.
// The caller must validate the returned access token and application profile
// before returning any tokens to a client or committing a web session.
func (k *Keycloak) Refresh(ctx context.Context, raw string) (SessionTokens, error) {
	if !validRefresh(raw) {
		return SessionTokens{}, ErrInvalid
	}

	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {raw}}
	status, body, err := k.sessionPost(ctx, "token", form)
	if err != nil {
		return SessionTokens{}, err
	}

	if status != http.StatusOK {
		var problem struct {
			Error string `json:"error"`
		}
		if (status == 400 ||
			status == 401) &&
			json.Unmarshal(body, &problem) == nil &&
			problem.Error == "invalid_grant" {
			return SessionTokens{}, ErrFailed
		}

		return SessionTokens{}, ErrUnavailable
	}

	var result struct {
		Access         string `json:"access_token"`
		Refresh        string `json:"refresh_token"`
		Type           string `json:"token_type"`
		Expires        int64  `json:"expires_in"`
		RefreshExpires int64  `json:"refresh_expires_in"`
	}
	if json.Unmarshal(body, &result) != nil ||
		!validRefresh(result.Access) ||
		!validRefresh(result.Refresh) ||
		!strings.EqualFold(result.Type, "Bearer") ||
		result.Expires <= 0 ||
		result.Expires > 3600 ||
		result.RefreshExpires <= 0 {
		return SessionTokens{}, ErrUnavailable
	}

	return SessionTokens{
		AccessToken:      result.Access,
		RefreshToken:     result.Refresh,
		ExpiresIn:        result.Expires,
		RefreshExpiresIn: result.RefreshExpires,
	}, nil
}

// RevokeRefresh uses RFC 7009, not the legacy refresh-token logout endpoint.
// Success is deliberately idempotent and does not prove that a token existed,
// or that all Keycloak sessions or offline-validated access JWTs are invalid.
func (k *Keycloak) RevokeRefresh(ctx context.Context, raw string) error {
	if !validRefresh(raw) {
		return ErrInvalid
	}

	status, _, err := k.sessionPost(ctx, "revoke", url.Values{"token": {raw}, "token_type_hint": {"refresh_token"}})
	if err != nil {
		return err
	}

	if status != http.StatusOK {
		return ErrUnavailable
	}

	return nil
}

func (k *Keycloak) sessionPost(ctx context.Context, endpoint string, form url.Values) (int, []byte, error) {
	form.Set("client_id", k.cfg.ClientID)
	form.Set("client_secret", k.cfg.ClientSecret)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		k.cfg.Issuer+"/protocol/openid-connect/"+endpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return 0, nil, ErrUnavailable
	}

	req.GetBody = nil
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	response, err := k.client.Do(req)
	if err != nil {
		return 0, nil, ErrUnavailable
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, nil, ErrUnavailable
	}

	return response.StatusCode, body, nil
}
