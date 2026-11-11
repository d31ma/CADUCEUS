// @Service
type SidebarLoadService struct {
	// @Needs
	messaging *MessagingClient
	// @Needs
	channelsClient *ChannelsClient
	// @Needs
	token *ConversationTokenService
}

func (loader *SidebarLoadService) DomainId() string {
	return loader.token.DomainId()
}

func (loader *SidebarLoadService) Channels() []Channel {
	return loader.channelsOrNone(loader.channels())
}

func (loader *SidebarLoadService) Summaries() ([]ConversationSummary, error) {
	return loader.summaries()
}

func (loader *SidebarLoadService) channels() ([]Channel, error) {
	channels := []Channel{}
	failure := loader.token.WithRenewal(func(authorization string) error {
		answer, failure := BehaviorsAnswer[[]Channel](loader.channelsClient.List(authorization))
		channels = answer
		return failure
	})
	return channels, failure
}

func (loader *SidebarLoadService) channelsOrNone(channels []Channel, failure error) []Channel {
	if failure != nil {
		return []Channel{}
	}
	return append([]Channel{}, channels...)
}

func (loader *SidebarLoadService) summaries() ([]ConversationSummary, error) {
	page := ConversationPage{}
	failure := loader.token.WithRenewal(func(authorization string) error {
		answer, failure := loader.messaging.Conversations(authorization, "", loader.pageSize())
		page = answer
		return failure
	})
	return append([]ConversationSummary{}, page.Items...), failure
}

func (loader *SidebarLoadService) pageSize() int {
	return 50
}
