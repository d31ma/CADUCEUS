import "mesail.dev/packages/channels"

// @Service
type ChannelWireService struct {
}

func (service *ChannelWireService) Channels(documents []channels.ChannelDocument) []Channel {
	return service.channels(documents)
}

func (service *ChannelWireService) Channel(document channels.ChannelDocument) Channel {
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

func (service *ChannelWireService) channels(documents []channels.ChannelDocument) []Channel {
	wire := []Channel{}
	for _, document := range documents {
		wire = append(wire, service.Channel(document))
	}
	return wire
}
