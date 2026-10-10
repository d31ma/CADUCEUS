import "net/http"

// @RequestRoute("/signatures/{id}")
// @SignedIn
type SignaturesIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	signaturesHelper *SignaturesHelper
	// @Needs
	usersHelper *UsersHelper
}

func (controller *SignaturesIdController) route() SignaturesRoute {
	return SignaturesRoute{Access: controller.accessHelper, Signatures: controller.signaturesHelper, Users: controller.usersHelper}
}

// @PutRoute
func (controller *SignaturesIdController) PutSignature(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Update(id, request)
}

// @DeleteRoute
func (controller *SignaturesIdController) DeleteSignature(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Delete(id, request)
}
