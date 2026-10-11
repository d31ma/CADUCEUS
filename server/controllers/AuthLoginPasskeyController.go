import "net/http"

// AuthLoginPasskeyController answers /auth/login/passkey. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/passkey")
// @Public
type AuthLoginPasskeyController struct {
	// @Needs
	rateLimitService *RateLimitService
}

// PostLoginPasskey answers POST /auth/login/passkey: supply a WebAuthn assertion.
// @PostRoute
func (controller *AuthLoginPasskeyController) PostLoginPasskey(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.passkey", Fields: []string{"transactionId", "credentialId", "authenticatorData", "clientDataJSON", "signature"}, Limit: &AuthLimit{Name: "login-passkey", Attempts: 12, WindowMinutes: 15}, Limiter: controller.rateLimitService}.Answer(request)
}
