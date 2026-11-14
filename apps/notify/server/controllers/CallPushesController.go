// @RequestRoute("/api/v1/notify/call-pushes")
// @Allow(Service)
type CallPushesController struct {
	// @Needs
	pushes *CallPushesService
}

// @PostRoute
func (controller *CallPushesController) PostCallPush(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CallPush,
) (CallPushReport, error) {
	return controller.pushes.Request(authorization, request)
}
