import "net/http"

// AuthJwksController answers /auth/jwks. It holds no authentication logic: the
// boundary decides, and the route only checks the request's shape.
// @RequestRoute("/auth/jwks")
// @Public
type AuthJwksController struct{}

// GetJwks answers GET /auth/jwks: the engine's public signing keys, so a client or an
// external verifier can check a CADUCEUS token. It carries no private material.
// @GetRoute
func (*AuthJwksController) GetJwks(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.jwks"}.Answer(request)
}
