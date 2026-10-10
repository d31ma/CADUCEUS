import "net/http"

// CaduceusIdentityProvider tells Tachyon who is signed in before a @SignedIn
// route runs. The Go authorization boundary verifies the bearer token.
// @IdentityProvider
type CaduceusIdentityProvider struct{}

func (provider *CaduceusIdentityProvider) Identify(request *http.Request) map[string]any {
	return SessionIdentity{}.Of(request)
}
