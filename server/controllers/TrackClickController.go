import "net/http"

// @RequestRoute("/track/click")
// @Public
type TrackClickController struct {
	// @Needs
	trackingHelper *TrackingHelper
}

// @GetRoute
func (controller *TrackClickController) GetTrackClick(request *http.Request) *http.Response {
	return TrackingRoute{Tracking: controller.trackingHelper}.Click(request)
}
