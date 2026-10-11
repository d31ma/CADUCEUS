import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// TestSeedUserRoute answers POST /test/seed/user: it provisions one mailbox
// and answers a real token for it. The token comes from the engine through
// the same login the product uses, so the harness proves the boundary that
// verifies it. It answers only when the deployment enables the test routes.
type TestSeedUserRoute struct {
	Users *UsersHelper
}

// Seed provisions the mailbox the body describes and signs it in.
func (route TestSeedUserRoute) Seed(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	if !(Security{}).TestRoutesEnabled() {
		return Answer{}.Forbidden("Only available in test mode")
	}
	domains, _ := RecordValues{}.List(body["domains"])
	if !(RecordValues{}).Truthy(body["email"]) || len(domains) == 0 || !(RecordValues{}).Truthy(body["role"]) {
		return Answer{}.BadRequest("email, domains, and role required")
	}
	address := strings.ToLower(BodyText{}.String(body["email"]))
	seeded, password, err := route.bootstrap(address, body["role"] == "admin")
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	codes := TextList{}.Of(seeded["recoveryCodes"])
	if len(codes) == 0 {
		return RouteAccess{}.Failure(errors.New("the engine issued no recovery codes"))
	}
	tokens, err := route.login(address, password, codes[0])
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	profile := StoredRecords{}.Without(body)
	profile["email"] = address
	route.copied(profile, seeded, "principalId", "tenantId")
	if _, err := route.Users.UpsertByEmail(profile); err != nil {
		return RouteAccess{}.Failure(err)
	}
	answer := map[string]any{
		"email": address, "password": password, "name": RecordValues{}.Or(body["name"], nil), "role": body["role"],
		"domains": []any{seeded["domain"]}, "recoveryCodes": codes[1:],
	}
	route.copied(answer, map[string]any{"token": tokens["accessToken"], "refreshToken": tokens["refreshToken"], "idToken": tokens["idToken"]}, "token", "refreshToken", "idToken")
	return Answer{}.Ok(answer)
}

// bootstrap provisions the mailbox with its own random password.
func (route TestSeedUserRoute) bootstrap(address string, admin bool) (map[string]any, string, error) {
	secret := make([]byte, route.passwordBytes())
	if _, err := rand.Read(secret); err != nil {
		return nil, "", err
	}
	password := base64.RawURLEncoding.EncodeToString(secret)
	role := "domain-viewer"
	if admin {
		role = "domain-admin"
	}
	seeded, err := route.call("auth.test.bootstrap", map[string]any{"address": address, "password": password, "role": role})
	return seeded, password, err
}

func (TestSeedUserRoute) passwordBytes() int { return 24 }

// login drives one complete authentication: the password, a recovery code
// as the second factor, then completion.
func (route TestSeedUserRoute) login(address string, password string, recoveryCode string) (map[string]any, error) {
	begun, err := route.call("auth.login.begin", map[string]any{"address": address})
	if err != nil {
		return nil, err
	}
	transaction := begun["transactionId"]
	if _, err := route.call("auth.login.password", map[string]any{"transactionId": transaction, "password": password}); err != nil {
		return nil, err
	}
	if _, err := route.call("auth.login.recovery", map[string]any{"transactionId": transaction, "code": recoveryCode}); err != nil {
		return nil, err
	}
	return route.call("auth.login.complete", map[string]any{"transactionId": transaction})
}

func (TestSeedUserRoute) call(operation string, parameters map[string]any) (map[string]any, error) {
	raw, err := AuthClient{Boundary: Boundary{}.FromSettings()}.Call(operation, parameters)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	_ = json.Unmarshal(raw, &result)
	return result, nil
}

// copied copies the named fields that source holds; a missing one stays
// missing, as an undefined field did.
func (TestSeedUserRoute) copied(target map[string]any, source map[string]any, fields ...string) {
	for _, field := range fields {
		if value, present := source[field]; present && value != nil {
			target[field] = value
		}
	}
}
