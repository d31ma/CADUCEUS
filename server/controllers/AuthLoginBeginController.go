import "net/http"

// AuthLoginBeginController answers /auth/login/begin. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/begin")
// @Public
type AuthLoginBeginController struct {
	// @Needs
	rateLimitService *RateLimitService
}

// PostLoginBegin answers POST /auth/login/begin: start one authentication transaction.
// @PostRoute
func (controller *AuthLoginBeginController) PostLoginBegin(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.begin", Fields: []string{"address"}, Limit: &AuthLimit{Name: "login-begin", Attempts: 20, WindowMinutes: 15}, Limiter: controller.rateLimitService}.Answer(request)
}
