import "net/http"

// @RequestRoute("/translation-assets")
// @SignedIn
type TranslationAssetsController struct {
	// @Needs
	accessHelper *AccessHelper
}

// @GetRoute
func (controller *TranslationAssetsController) GetAsset(request *http.Request) *http.Response {
	return TranslationAssetsRoute{Access: controller.accessHelper}.Read(request)
}
