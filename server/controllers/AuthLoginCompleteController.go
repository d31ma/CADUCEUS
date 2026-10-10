import "net/http"

// AuthLoginCompleteController answers /auth/login/complete. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/login/complete")
// @Public
type AuthLoginCompleteController struct{}

// PostLoginComplete answers POST /auth/login/complete: issue the session and its token set.
// @PostRoute
func (*AuthLoginCompleteController) PostLoginComplete(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.login.complete", Fields: []string{"transactionId"}}.Answer(request)
}
