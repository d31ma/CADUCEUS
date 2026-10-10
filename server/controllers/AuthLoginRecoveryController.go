import "net/http"

// AuthLoginRecoveryController answers /auth/login/recovery. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/recovery")
// @Public
type AuthLoginRecoveryController struct {
	// @Needs
	rateLimitService *RateLimitService
}

// PostLoginRecovery answers POST /auth/login/recovery: supply a single-use recovery code.
// @PostRoute
func (controller *AuthLoginRecoveryController) PostLoginRecovery(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.recovery", Fields: []string{"transactionId", "code"}, Limit: &AuthLimit{Name: "login-recovery", Attempts: 10, WindowMinutes: 15}, Limiter: controller.rateLimitService}.Answer(request)
}
