import "net/http"

// AuthLoginPasswordController answers /auth/login/password. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/password")
// @Public
type AuthLoginPasswordController struct {
	// @Needs
	rateLimitService *RateLimitService
}

// PostLoginPassword answers POST /auth/login/password: supply the password factor.
// @PostRoute
func (controller *AuthLoginPasswordController) PostLoginPassword(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.password", Fields: []string{"transactionId", "password"}, Limit: &AuthLimit{Name: "login-password", Attempts: 12, WindowMinutes: 15}, Limiter: controller.rateLimitService}.Answer(request)
}
