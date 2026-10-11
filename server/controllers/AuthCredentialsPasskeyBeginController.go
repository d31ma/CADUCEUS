import "net/http"

// AuthCredentialsPasskeyBeginController answers /auth/credentials/passkey/begin. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/credentials/passkey/begin")
// @Public
type AuthCredentialsPasskeyBeginController struct{}

// PostPasskeyBegin answers POST /auth/credentials/passkey/begin: a challenge to register a passkey. Changing a
// credential needs a freshly satisfied authentication transaction, which the
// boundary proves against the same principal.
// @PostRoute
func (*AuthCredentialsPasskeyBeginController) PostPasskeyBegin(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.passkey.begin", Fields: []string{"stepUpTransactionId"}, Signed: true}.Answer(request)
}
