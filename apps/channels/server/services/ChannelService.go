import (
	"mesail.dev/packages/channels"
	"mesail.dev/packages/outbox"
)

// @Service
type ChannelService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	wire *ChannelWireService
	// @Needs
	failures *ErrorMappingService
	pusher   outbox.Pusher
	lookup   channels.Directory
}

func (service *ChannelService) List(authorization string) ([]Channel, error) {
	documents, err := service.workflow().List(authorization)
	return service.wire.Channels(documents), service.failures.ChannelError(err)
}

func (service *ChannelService) Create(authorization string, request CreateChannelRequest) (Channel, error) {
	terms := channels.CreateRequest{Name: request.Name, Topic: request.Topic, Visibility: request.Visibility}
	document, err := service.workflow().Create(authorization, terms)
	return service.wire.Channel(document), service.failures.ChannelError(err)
}

func (service *ChannelService) Join(authorization string, id string) error {
	return service.failures.ChannelError(service.workflow().Join(authorization, id))
}

func (service *ChannelService) Leave(authorization string, id string) error {
	return service.failures.ChannelError(service.workflow().Leave(authorization, id))
}

func (service *ChannelService) Invite(authorization string, id string, request AddMembersRequest) error {
	return service.failures.ChannelError(service.workflow().Invite(authorization, id, request.MemberIds))
}

func (service *ChannelService) Update(authorization string, id string, request UpdateChannelRequest) (Channel, error) {
	changes := channels.UpdateRequest{Topic: request.Topic, EmailAddress: request.EmailAddress, Archived: request.Archived}
	document, err := service.workflow().Update(authorization, id, changes)
	return service.wire.Channel(document), service.failures.ChannelError(err)
}

func (service *ChannelService) workflow() channels.Workflow {
	return channels.Assembly{Source: service.dataSource, Pusher: service.pusher, Directory: service.lookup}.Workflow()
}
