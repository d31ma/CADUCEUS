import "net/http"

// @RequestRoute("/signatures")
// @SignedIn
type SignaturesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	signaturesHelper *SignaturesHelper
	// @Needs
	usersHelper *UsersHelper
}

func (controller *SignaturesController) route() SignaturesRoute {
	return SignaturesRoute{Access: controller.accessHelper, Signatures: controller.signaturesHelper, Users: controller.usersHelper}
}

// @PostRoute
func (controller *SignaturesController) PostSignature(request *http.Request) *http.Response {
	return controller.route().Create(request)
}

// @GetRoute
func (controller *SignaturesController) GetSignatures(request *http.Request) *http.Response {
	return controller.route().List(request)
}
