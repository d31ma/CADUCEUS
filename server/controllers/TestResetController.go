import "net/http"

// TestResetController answers /test/reset, which wipes every FYLO
// collection. It answers only when the deployment enables the test routes.
// @RequestRoute("/test/reset")
// @Public
type TestResetController struct {
	// @Needs
	fyloHelper *FyloHelper
}

// DeleteEverything wipes every collection.
// @DeleteRoute
func (controller *TestResetController) DeleteEverything(request *http.Request) *http.Response {
	return TestResetRoute{Fylo: controller.fyloHelper}.Reset(request)
}
