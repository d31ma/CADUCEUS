import "net/http"

// @RequestRoute("/suppressed/{address}")
// @SignedIn
type SuppressedAddressController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	suppressedHelper *SuppressedHelper
}

// @DeleteRoute
func (controller *SuppressedAddressController) DeleteAddress(
	// @FromPath
	address string,
	request *http.Request,
) *http.Response {
	return SuppressedRoute{Access: controller.accessHelper, Suppressed: controller.suppressedHelper}.Remove(address, request)
}
