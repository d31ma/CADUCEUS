import (
	"net/http"

	"mesail.dev/packages/oidc"
)

// @Service
type CallerService struct {
}

func (callers *CallerService) From(request *http.Request) (oidc.Caller, error) {
	return oidc.Configured().Identify(request.Header.Get("Authorization"))
}
