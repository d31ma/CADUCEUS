import "net/http"

// TestSeedTrackingController answers /test/seed/tracking, which records one
// recipient signal on the caller's sent message. It answers only when the
// deployment enables the test routes.
// @RequestRoute("/test/seed/tracking")
// @SignedIn
type TestSeedTrackingController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	trackingHelper *TrackingHelper
}

// PostSignal records the signal.
// @PostRoute
func (controller *TestSeedTrackingController) PostSignal(request *http.Request) *http.Response {
	return TestSeedTrackingRoute{Access: controller.accessHelper, Emails: controller.emailsHelper, Tracking: controller.trackingHelper}.Record(request)
}
