import "net/http"

// AuthCredentialsTotpEnrollController answers /auth/credentials/totp/enroll. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/credentials/totp/enroll")
// @Public
type AuthCredentialsTotpEnrollController struct{}

// PostTotpEnroll answers POST /auth/credentials/totp/enroll: issue a TOTP secret, once, after a
// freshly satisfied authentication transaction.
// @PostRoute
func (*AuthCredentialsTotpEnrollController) PostTotpEnroll(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.totp.enroll", Fields: []string{"stepUpTransactionId"}, Signed: true}.Answer(request)
}
