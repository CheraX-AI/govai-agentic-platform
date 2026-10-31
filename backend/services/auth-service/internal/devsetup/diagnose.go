package devsetup

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"net/url"
	"path/filepath"
)

func (a *localAdmin) diagnose(ctx context.Context, root string) error {
	env, e := godotenv.Read(filepath.Join(root, "..", "..", "services", "auth-service", ".env.registration"))
	if e != nil {
		return ErrSetup
	}
	// No administrative Authorization header on the service-client token request.
	token := a.token
	a.token = ""
	status, b := a.request(ctx, "POST", "/realms/GovAI/protocol/openid-connect/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {env["REGISTRATION_CLIENT_ID"]}, "client_secret": {env["REGISTRATION_CLIENT_SECRET"]}})
	a.token = token
	fmt.Fprintf(a.out, "DEV_AUTH_SERVICE_TOKEN status=%d\n", status)
	if status != 200 {
		v := mustJSON[object](b)
		switch v["error"] {
		case "invalid_client", "unauthorized_client", "invalid_grant":
			fmt.Fprintln(a.out, "DEV_AUTH_SERVICE_TOKEN_REJECTED")
		}
		return ErrSetup
	}
	clientID := a.clientID(ctx, registrationClient)
	principal := a.get(ctx, realmPath+"/clients/"+clientID+"/service-account-user")["id"]
	permissionBase := realmPath + "/clients/" + a.clientID(ctx, "admin-permissions") + "/authz/resource-server"
	policies := a.list(ctx, permissionBase+"/policy?name=govai-dev-registration-principal")
	if len(policies) != 1 {
		return ErrSetup
	}
	policy := a.get(ctx, permissionBase+"/policy/user/"+policies[0]["id"].(string))
	users, ok := policy["users"].([]any)
	matched := ok && len(users) == 1 && users[0] == principal
	fmt.Fprintf(a.out, "DEV_AUTH_POLICY_PRINCIPAL matched=%t\n", matched)
	permissions := a.list(ctx, permissionBase+"/permission?name=govai-dev-registration-group")
	if len(permissions) != 1 {
		return ErrSetup
	}
	permission := a.get(ctx, permissionBase+"/permission/scope/"+permissions[0]["id"].(string))
	permissionPath := permissionBase + "/policy/" + permission["id"].(string)
	resources := a.list(ctx, permissionPath+"/resources")
	scopes := a.list(ctx, permissionPath+"/scopes")
	pols := a.list(ctx, permissionPath+"/associatedPolicies")
	fmt.Fprintf(a.out, "DEV_AUTH_GROUP_PERMISSION resources=%d scopes=%d policies=%d\n", len(resources), len(scopes), len(pols))
	if !matched || len(resources) != 1 || len(scopes) != 4 || len(pols) != 1 {
		return ErrSetup
	}
	return nil
}
