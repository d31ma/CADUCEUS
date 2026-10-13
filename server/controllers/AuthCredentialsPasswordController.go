import "net/http"

// AuthCredentialsPasswordController answers /auth/credentials/password. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/credentials/password")
// @Public
type AuthCredentialsPasswordController struct{}

// PutPassword answers PUT /auth/credentials/password: set the caller's password after a
// freshly satisfied authentication transaction.
// @PutRoute
func (*AuthCredentialsPasswordController) PutPassword(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.password.set", Fields: []string{"stepUpTransactionId", "password"}, Signed: true}.Answer(request)
}
