import "net/http"

// @RequestRoute("/send/tick")
// @Public
type SendTickController struct {
	// @Needs
	outboundQueueService *OutboundQueueService
}

// @PostRoute
func (controller *SendTickController) PostTick(request *http.Request) *http.Response {
	return SendQueueRoute{Queue: controller.outboundQueueService}.Tick(request)
}
