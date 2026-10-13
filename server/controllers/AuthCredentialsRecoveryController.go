import "net/http"

// AuthCredentialsRecoveryController answers /auth/credentials/recovery. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/credentials/recovery")
// @Public
type AuthCredentialsRecoveryController struct{}

// PostRecoveryCodes answers POST /auth/credentials/recovery: replace the caller's recovery codes after
// a freshly satisfied authentication transaction.
// @PostRoute
func (*AuthCredentialsRecoveryController) PostRecoveryCodes(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.recovery.issue", Fields: []string{"stepUpTransactionId"}, Signed: true}.Answer(request)
}
