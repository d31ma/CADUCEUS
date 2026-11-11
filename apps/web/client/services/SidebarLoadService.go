import "errors"

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
	return loader.pagesFrom("", loader.pageLimit())
}

func (loader *SidebarLoadService) pagesFrom(cursor string, pagesLeft int) ([]ConversationSummary, error) {
	page, failure := loader.page(cursor)
	rest, restFailure := loader.restAfter(page.NextCursor, pagesLeft-1, failure)
	return append(append([]ConversationSummary{}, page.Items...), rest...), errors.Join(failure, restFailure)
}

func (loader *SidebarLoadService) restAfter(cursor string, pagesLeft int, failure error) ([]ConversationSummary, error) {
	if failure != nil || cursor == "" || pagesLeft == 0 {
		return nil, nil
	}
	return loader.pagesFrom(cursor, pagesLeft)
}

func (loader *SidebarLoadService) page(cursor string) (ConversationPage, error) {
	page := ConversationPage{}
	failure := loader.token.WithRenewal(func(authorization string) error {
		answer, failure := loader.messaging.Conversations(authorization, cursor, loader.pageSize())
		page = answer
		return failure
	})
	return page, failure
}

func (loader *SidebarLoadService) pageSize() int {
	return 50
}

func (loader *SidebarLoadService) pageLimit() int {
	return 4
}
