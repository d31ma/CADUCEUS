import "net/http"

// DomainsDomainRoutesController answers /domains/{domain}/routes: the
// inbound routing rules of one hosted domain.
// @RequestRoute("/domains/{domain}/routes")
// @SignedIn
type DomainsDomainRoutesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	domainsHelper *DomainsHelper
}

// GetRoutes answers the domain's routing rules.
// @GetRoute
func (controller *DomainsDomainRoutesController) GetRoutes(
	// @FromPath
	domain string,
	request *http.Request,
) *http.Response {
	return DomainRoutesRoute{Access: controller.accessHelper, Domains: controller.domainsHelper}.List(domain, request)
}
