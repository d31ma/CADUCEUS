import (
	"errors"

	"mesail.dev/packages/channels"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/channels")
// @SignedIn
type ChannelsController struct {
	// @Needs
	dataSource DataSource
	pusher     outbox.Pusher
	lookup     channels.Directory
}

// @GetRoute
func (controller *ChannelsController) GetChannels(
	// @FromHeader("authorization")
	authorization string,
) ([]Channel, error) {
	documents, err := controller.workflow().List(authorization)
	return controller.wireChannels(documents), controller.failure(err)
}

// @PostRoute
// @ResponseStatus(201)
func (controller *ChannelsController) PostChannel(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateChannelRequest,
) (Channel, error) {
	terms := channels.CreateRequest{Name: request.Name, Topic: request.Topic, Visibility: request.Visibility}
	document, err := controller.workflow().Create(authorization, terms)
	return controller.wireChannel(document), controller.failure(err)
}

// @PostRoute("/{id}/join")
// @ResponseStatus(204)
func (controller *ChannelsController) PostJoin(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	id string,
) error {
	return controller.failure(controller.workflow().Join(authorization, id))
}

// @PostRoute("/{id}/leave")
// @ResponseStatus(204)
func (controller *ChannelsController) PostLeave(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	id string,
) error {
	return controller.failure(controller.workflow().Leave(authorization, id))
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
	return controller.failure(controller.workflow().Invite(authorization, id, request.MemberIds))
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
	changes := channels.UpdateRequest{Topic: request.Topic, EmailAddress: request.EmailAddress, Archived: request.Archived}
	document, err := controller.workflow().Update(authorization, id, changes)
	return controller.wireChannel(document), controller.failure(err)
}

func (controller *ChannelsController) workflow() channels.Workflow {
	return channels.Assembly{Source: controller.dataSource, Pusher: controller.pusher, Directory: controller.lookup}.Workflow()
}

func (controller *ChannelsController) wireChannels(documents []channels.ChannelDocument) []Channel {
	wire := []Channel{}
	for _, document := range documents {
		wire = append(wire, controller.wireChannel(document))
	}
	return wire
}

func (controller *ChannelsController) wireChannel(document channels.ChannelDocument) Channel {
	return Channel{
		ChannelId:      channels.WireIds{}.ChannelId(document.Id),
		Name:           document.Name,
		Topic:          document.Topic,
		Visibility:     document.Visibility,
		EmailAddress:   document.EmailAddress,
		Archived:       document.Archived,
		ConversationId: document.ConversationId,
	}
}

func (controller *ChannelsController) failure(failed error) error {
	invalid := &channels.InvalidRequest{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Message: invalid.Message, Field: invalid.Field}
	case errors.As(failed, new(*channels.Unauthenticated)):
		return &Unauthenticated{Message: failed.Error()}
	case errors.As(failed, new(*channels.Forbidden)):
		return &Forbidden{Message: failed.Error()}
	case errors.As(failed, new(*channels.NotFound)):
		return &NotFound{Message: failed.Error()}
	case errors.As(failed, new(*channels.Conflict)):
		return &Conflict{Message: failed.Error()}
	case errors.As(failed, new(*channels.Duplicate)):
		return &Duplicate{Message: failed.Error()}
	}
	return &Unavailable{Message: "the channel service is not available"}
}
