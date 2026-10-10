import "net/http"

// @RequestRoute("/domains")
// @SignedIn
type DomainsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	domainsHelper *DomainsHelper
}

// @GetRoute
func (controller *DomainsController) GetDomains(request *http.Request) *http.Response {
	return controller.route().List(request)
}

// @PostRoute
func (controller *DomainsController) PostDomain(request *http.Request) *http.Response {
	return controller.route().Create(request)
}

func (controller *DomainsController) route() DomainsRoute {
	return DomainsRoute{Access: controller.accessHelper, Domains: controller.domainsHelper}
}
