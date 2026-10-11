import "net/http"

// AuthTokenRefreshController answers /auth/token/refresh. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/token/refresh")
// @Public
type AuthTokenRefreshController struct{}

// PostTokenRefresh answers POST /auth/token/refresh: rotate one refresh token.
// @PostRoute
func (*AuthTokenRefreshController) PostTokenRefresh(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.token.refresh", Fields: []string{"refreshToken", "clientId"}}.Answer(request)
}
