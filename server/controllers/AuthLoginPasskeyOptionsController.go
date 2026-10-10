import "net/http"

// AuthLoginPasskeyOptionsController answers /auth/login/passkey/options. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/passkey/options")
// @Public
type AuthLoginPasskeyOptionsController struct{}

// PostPasskeyOptions answers POST /auth/login/passkey/options: the challenge for an in-flight login.
// @PostRoute
func (*AuthLoginPasskeyOptionsController) PostPasskeyOptions(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.passkey.options", Fields: []string{"transactionId"}}.Answer(request)
}
