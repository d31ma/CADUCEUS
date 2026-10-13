import "net/http"

// @RequestRoute("/send/flush")
// @Public
type SendFlushController struct {
	// @Needs
	outboundQueueService *OutboundQueueService
}

// @PostRoute
func (controller *SendFlushController) PostFlush(request *http.Request) *http.Response {
	return SendQueueRoute{Queue: controller.outboundQueueService}.Flush(request)
}
