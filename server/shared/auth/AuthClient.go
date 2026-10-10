import (
	"encoding/json"
	"time"
)

// AuthClient carries one authentication or authorization operation to the
// engine that the Go owner runs. It decides nothing itself: every credential,
// session and permission answer comes from the boundary.
type AuthClient struct {
	Boundary Boundary
}

// Call runs one composed operation, such as auth.token.authenticate.
func (client AuthClient) Call(operation string, parameters map[string]any) (json.RawMessage, error) {
	return client.Boundary.Call(map[string]any{"op": operation, "params": parameters}, client.responseLimit(), client.timeout())
}

func (AuthClient) responseLimit() int { return 1 << 20 }

func (AuthClient) timeout() time.Duration { return 15 * time.Second }

// Authenticate answers the principal a bearer token belongs to.
func (client AuthClient) Authenticate(token string) (Principal, error) {
	var principal Principal
	raw, err := client.Call("auth.token.authenticate", map[string]any{"token": token})
	if err != nil {
		return principal, err
	}
	return principal, json.Unmarshal(raw, &principal)
}

// Authorize asks whether the token's principal may act on a target, such as
// {"mailbox": "ada@example.com"} or {"domain": "example.com"}.
func (client AuthClient) Authorize(token string, action string, target map[string]string) (AuthDecision, error) {
	var decision AuthDecision
	parameters := map[string]any{"token": token, "action": action}
	for key, value := range target {
		parameters[key] = value
	}
	raw, err := client.Call("auth.request.authorize", parameters)
	if err != nil {
		return decision, err
	}
	return decision, json.Unmarshal(raw, &decision)
}
