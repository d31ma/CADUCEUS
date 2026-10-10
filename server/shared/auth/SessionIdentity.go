import "net/http"

// SessionIdentity tells Tachyon who is signed in: the principal the boundary
// recognises the bearer token as, or nil, which Tachyon answers with 401.
type SessionIdentity struct{}

// Of answers the identity of a request.
func (SessionIdentity) Of(request *http.Request) map[string]any {
	access := RouteAccess{Auth: AuthClient{Boundary: Boundary{}.FromSettings()}}
	token := access.Bearer(request)
	if token == "" {
		return nil
	}
	principal, err := access.Auth.Authenticate(token)
	if err != nil {
		return nil
	}
	return map[string]any{"user": principal.Address, "roles": []string{}}
}
