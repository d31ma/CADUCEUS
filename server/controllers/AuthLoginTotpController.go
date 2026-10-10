import "net/http"

// AuthLoginTotpController answers /auth/login/totp. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/totp")
// @Public
type AuthLoginTotpController struct {
	// @Needs
	rateLimitService *RateLimitService
}

// PostLoginTotp answers POST /auth/login/totp: supply a time-based code.
// @PostRoute
func (controller *AuthLoginTotpController) PostLoginTotp(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.totp", Fields: []string{"transactionId", "code"}, Limit: &AuthLimit{Name: "login-totp", Attempts: 10, WindowMinutes: 5}, Limiter: controller.rateLimitService}.Answer(request)
}
