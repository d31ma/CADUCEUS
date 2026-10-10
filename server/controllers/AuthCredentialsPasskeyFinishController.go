import "net/http"

// AuthCredentialsPasskeyFinishController answers /auth/credentials/passkey/finish. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/credentials/passkey/finish")
// @Public
type AuthCredentialsPasskeyFinishController struct{}

// PostPasskeyFinish answers POST /auth/credentials/passkey/finish: store a verified passkey.
// @PostRoute
func (*AuthCredentialsPasskeyFinishController) PostPasskeyFinish(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.passkey.finish", Fields: []string{"attestationObject", "clientDataJSON"}, Signed: true}.Answer(request)
}
