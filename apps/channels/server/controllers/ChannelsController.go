// @RequestRoute("/api/v1/channels")
// @SignedIn
type ChannelsController struct {
	// @Needs
	channels *ChannelService
}

// @GetRoute
func (controller *ChannelsController) GetChannels(
	// @FromHeader("authorization")
	authorization string,
) ([]Channel, error) {
	return controller.channels.List(authorization)
}

// @PostRoute
// @ResponseStatus(201)
func (controller *ChannelsController) PostChannel(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateChannelRequest,
) (Channel, error) {
	return controller.channels.Create(authorization, request)
}

// @PostRoute("/{id}/join")
// @ResponseStatus(204)
func (controller *ChannelsController) PostJoin(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	id string,
) error {
	return controller.channels.Join(authorization, id)
}

// @PostRoute("/{id}/leave")
// @ResponseStatus(204)
func (controller *ChannelsController) PostLeave(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	id string,
) error {
	return controller.channels.Leave(authorization, id)
}

// @PostRoute("/{id}/invite")
// @ResponseStatus(204)
func (controller *ChannelsController) PostInvite(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	id string,
	// @FromBody
	request AddMembersRequest,
) error {
	return controller.channels.Invite(authorization, id, request)
}

// @PatchRoute("/{id}")
func (controller *ChannelsController) PatchChannel(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	id string,
	// @FromBody
	request UpdateChannelRequest,
) (Channel, error) {
	return controller.channels.Update(authorization, id, request)
}
