// @RequestRoute("/api/v1/realtime/presence")
// @SignedIn
type PresenceController struct {
	// @Needs
	presence *PresenceService
}

// @PutRoute
// @ResponseStatus(204)
func (controller *PresenceController) PutPresence(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request PresenceRequest,
) error {
	return controller.presence.Set(authorization, request)
}

// @GetRoute
func (controller *PresenceController) GetPresence(
	// @FromHeader("authorization")
	authorization string,
	// @FromQuery("memberIds")
	memberIds string,
) ([]Presence, error) {
	return controller.presence.Query(authorization, memberIds)
}
