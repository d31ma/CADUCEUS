import "net/http"

// @RequestRoute("/send")
// @SignedIn
type SendController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	sendService *SendService
	// @Needs
	sendIdempotencyService *SendIdempotencyService
	// @Needs
	rateLimitService *RateLimitService
}

// @PostRoute
func (controller *SendController) PostSend(request *http.Request) *http.Response {
	route := SendRoute{Access: controller.accessHelper, Send: controller.sendService, Idempotency: controller.sendIdempotencyService, Limits: controller.rateLimitService}
	return route.Create(request)
}
