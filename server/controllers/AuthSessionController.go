import "net/http"

// AuthSessionController answers /auth/session: the caller's own session.
// @RequestRoute("/auth/session")
// @Public
type AuthSessionController struct{}

// GetSession answers the caller's resolved principal, so a client can confirm
// a stored token still speaks for a live session.
// @GetRoute
func (*AuthSessionController) GetSession(request *http.Request) *http.Response {
	return AuthRelay{}.Session(request)
}

// DeleteSession ends the caller's own session.
// @DeleteRoute
func (*AuthSessionController) DeleteSession(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.session.revoke", Signed: true, Fixed: map[string]any{"reason": "signed out"}}.Answer(request)
}
