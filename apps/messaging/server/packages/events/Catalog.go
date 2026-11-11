package events

type Catalog struct{}

func (catalog Catalog) ProducerOf(eventType string) (string, error) {
	route, err := catalog.route(eventType)
	return route.Producer, err
}

func (catalog Catalog) ProducersOf(eventType string) ([]string, error) {
	route, err := catalog.route(eventType)
	return append([]string{route.Producer}, route.OtherProducers...), err
}

func (catalog Catalog) ConsumersOf(eventType string) []string {
	route, _ := catalog.route(eventType)
	return append([]string{}, route.Consumers...)
}

func (catalog Catalog) Known(eventType string) bool {
	_, err := catalog.route(eventType)
	return err == nil
}

func (catalog Catalog) route(eventType string) (Route, error) {
	route, known := catalog.table()[eventType]
	if !known {
		return Route{Consumers: []string{}}, UnknownEventType{Type: eventType}
	}
	return route, nil
}

func (Catalog) table() map[string]Route {
	types := Types{}
	services := Services{}
	channelConsumers := []string{services.Messaging(), services.Realtime(), services.Search()}
	messageConsumers := []string{services.Realtime(), services.Notify(), services.Search()}
	activityConsumers := []string{services.Realtime(), services.Notify()}
	return map[string]Route{
		types.MemberUpserted():       {Producer: services.Directory(), Consumers: []string{services.Messaging(), services.Notify(), services.Search()}},
		types.ChannelCreated():       {Producer: services.Channels(), Consumers: channelConsumers},
		types.ChannelMemberAdded():   {Producer: services.Channels(), Consumers: channelConsumers},
		types.ChannelMemberRemoved(): {Producer: services.Channels(), Consumers: channelConsumers},
		types.ChannelArchived():      {Producer: services.Channels(), Consumers: channelConsumers},
		types.MessageCreated():       {Producer: services.Messaging(), Consumers: messageConsumers},
		types.MessageEdited():        {Producer: services.Messaging(), Consumers: messageConsumers},
		types.MessageDeleted():       {Producer: services.Messaging(), Consumers: messageConsumers},
		types.ReactionChanged():      {Producer: services.Messaging(), Consumers: activityConsumers},
		types.ReadPositionMoved():    {Producer: services.Messaging(), Consumers: activityConsumers},
		types.EmailSendRequested():   {Producer: services.Messaging(), OtherProducers: []string{services.Notify()}, Consumers: []string{services.MailOut()}},
		types.EmailDeliveryChanged(): {Producer: services.MailOut(), Consumers: []string{services.Messaging()}},
		types.EmailReceived():        {Producer: services.MailIn(), Consumers: []string{services.Messaging()}},
		types.PresenceChanged():      {Producer: services.Realtime(), Consumers: []string{services.Realtime()}},
	}
}
