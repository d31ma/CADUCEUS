import "net/http"

// @RequestRoute("/track/open")
// @Public
type TrackOpenController struct {
	// @Needs
	trackingHelper *TrackingHelper
}

// @GetRoute
func (controller *TrackOpenController) GetTrackOpen(request *http.Request) *http.Response {
	return TrackingRoute{Tracking: controller.trackingHelper}.Open(request)
}
