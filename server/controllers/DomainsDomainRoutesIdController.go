import "net/http"

// DomainsDomainRoutesIdController answers /domains/{domain}/routes/{id}: one
// inbound routing rule of a domain the caller administers.
// @RequestRoute("/domains/{domain}/routes/{id}")
// @SignedIn
type DomainsDomainRoutesIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	domainsHelper *DomainsHelper
}

// DeleteRoute deletes the rule.
// @DeleteRoute
func (controller *DomainsDomainRoutesIdController) DeleteRoute(
	// @FromPath
	domain string,
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Remove(DomainRule{Domain: domain, Id: id}, request)
}

// PutRoute stores the rule under its id.
// @PutRoute
func (controller *DomainsDomainRoutesIdController) PutRoute(
	// @FromPath
	domain string,
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Replace(DomainRule{Domain: domain, Id: id}, request)
}

func (controller *DomainsDomainRoutesIdController) route() DomainRoutesRoute {
	return DomainRoutesRoute{Access: controller.accessHelper, Domains: controller.domainsHelper}
}
