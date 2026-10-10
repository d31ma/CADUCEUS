import "net/http"

// AuthLogoutController answers /auth/logout. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/logout")
// @Public
type AuthLogoutController struct{}

// PostLogout answers POST /auth/logout: end the session an ID token speaks for.
// @PostRoute
func (*AuthLogoutController) PostLogout(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.logout", Fields: []string{"idToken"}}.Answer(request)
}
