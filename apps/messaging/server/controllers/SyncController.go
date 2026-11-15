// @RequestRoute("/api/v1/messaging/sync")
// @SignedIn
type SyncController struct {
	// @Needs
	sync *SyncRouteService
}

// @PostRoute
func (controller *SyncController) PostSync(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request SyncRequest,
) (SyncResponse, error) {
	return controller.sync.Sync(authorization, request)
}
