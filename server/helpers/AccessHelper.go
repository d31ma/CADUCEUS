import "net/http"

// AccessHelper authorizes a request with the boundary and, once it is
// allowed, makes the request's storage act as the caller.
// @Helper
type AccessHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Signed answers the caller of any live session.
func (helper *AccessHelper) Signed(request *http.Request) (RouteCaller, *http.Response) {
	return helper.acting(helper.access().Signed(request))
}

// Allowed answers the caller when the boundary permits the action on the
// named domain or mailbox (or on what the caller owns when neither is named).
func (helper *AccessHelper) Allowed(request *http.Request, action string, domain string, mailbox string) (RouteCaller, *http.Response) {
	return helper.acting(helper.access().Allowed(request, action, domain, mailbox))
}

func (helper *AccessHelper) acting(caller RouteCaller, refusal *http.Response) (RouteCaller, *http.Response) {
	if refusal == nil {
		helper.fyloHelper.ActFor(caller.Email, caller.Domains)
	}
	return caller, refusal
}

func (*AccessHelper) access() RouteAccess {
	return RouteAccess{Auth: AuthClient{Boundary: Boundary{}.FromSettings()}}
}
