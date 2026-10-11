import "net/http"

// AuthCredentialsTotpActivateController answers /auth/credentials/totp/activate. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/credentials/totp/activate")
// @Public
type AuthCredentialsTotpActivateController struct{}

// PostTotpActivate answers POST /auth/credentials/totp/activate: prove a code and enable TOTP.
// @PostRoute
func (*AuthCredentialsTotpActivateController) PostTotpActivate(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.totp.activate", Fields: []string{"code"}, Signed: true}.Answer(request)
}
