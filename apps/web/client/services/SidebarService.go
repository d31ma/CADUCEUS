import "slices"

// @Service
type SidebarService struct {
	DomainId       string
	State          string
	Channels       []ConversationRow
	Directs        []ConversationRow
	Emails         []ConversationRow
	CurrentId      string
	ChannelsClosed bool
	DirectsClosed  bool
	EmailsClosed   bool
	channelInfo    []Channel
	// @Needs
	messaging *MessagingClient
	// @Needs
	channelsClient *ChannelsClient
	// @Needs
	token *ConversationTokenService
	// @Needs
	shell *ShellService
	// @Needs
	rows                      *SidebarRowService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (sidebar *SidebarService) Start() {
	sidebar.State = sidebar.stateLoading()
	sidebar.applicationEventPublisher.PublishEvent(ConversationDomainSelected{DomainId: sidebar.token.DomainId()}, true)
}

// @OnEvent({"replay":true})
// @InBackground
func (sidebar *SidebarService) DomainSelected(selected ConversationDomainSelected) {
	sidebar.clear(selected.DomainId)
	sidebar.load(selected.DomainId)
}

// @OnEvent
// @InBackground
func (sidebar *SidebarService) Live(event RealtimeEvent) {
	sidebar.refreshWhen(sidebar.wantsRefresh(event.Type))
}

func (sidebar *SidebarService) Open(conversationId string) {
	sidebar.openWhenKnown(sidebar.rowOf(conversationId))
}

func (sidebar *SidebarService) ToggleSection(section string) {
	sidebar.toggle(section)
}

func (sidebar *SidebarService) Retry() {
	sidebar.clear(sidebar.DomainId)
	sidebar.load(sidebar.DomainId)
}

func (sidebar *SidebarService) clear(domainId string) {
	sidebar.DomainId = domainId
	sidebar.State = sidebar.stateLoading()
	sidebar.Channels = []ConversationRow{}
	sidebar.Directs = []ConversationRow{}
	sidebar.Emails = []ConversationRow{}
	sidebar.CurrentId = ""
	sidebar.channelInfo = []Channel{}
}

func (sidebar *SidebarService) load(domainId string) {
	channels := sidebar.fetchChannels()
	summaries, failure := sidebar.fetchSummaries()
	sidebar.applyWhenCurrent(domainId, summaries, channels, failure)
}

func (sidebar *SidebarService) refresh(domainId string) {
	summaries, failure := sidebar.fetchSummaries()
	sidebar.applyLive(domainId, summaries, failure)
}

func (sidebar *SidebarService) fetchChannels() []Channel {
	authorization, failure := sidebar.token.Authorization()
	if failure != nil {
		return []Channel{}
	}
	return sidebar.channelsOrNone(BehaviorsAnswer[[]Channel](sidebar.channelsClient.List(authorization)))
}

func (sidebar *SidebarService) fetchSummaries() ([]ConversationSummary, error) {
	authorization, failure := sidebar.token.Authorization()
	if failure != nil {
		return []ConversationSummary{}, failure
	}
	page, failure := sidebar.messaging.Conversations(authorization, "", sidebar.pageSize())
	return page.Items, failure
}

func (sidebar *SidebarService) channelsOrNone(channels []Channel, failure error) []Channel {
	if failure != nil {
		return []Channel{}
	}
	return channels
}

func (sidebar *SidebarService) applyWhenCurrent(domainId string, summaries []ConversationSummary, channels []Channel, failure error) {
	if domainId != sidebar.DomainId {
		return
	}
	sidebar.apply(summaries, channels, failure)
}

func (sidebar *SidebarService) applyLive(domainId string, summaries []ConversationSummary, failure error) {
	if failure != nil || domainId != sidebar.DomainId {
		return
	}
	sidebar.apply(summaries, sidebar.channelInfo, failure)
}

func (sidebar *SidebarService) apply(summaries []ConversationSummary, channels []Channel, failure error) {
	rows := sidebar.rows.Rows(summaries, channels, sidebar.CurrentId)
	sidebar.channelInfo = channels
	sidebar.Channels = sidebar.rows.Section(rows, sidebar.sectionChannels())
	sidebar.Directs = sidebar.rows.Section(rows, sidebar.sectionDirect())
	sidebar.Emails = sidebar.rows.Section(rows, sidebar.sectionEmail())
	sidebar.State = sidebar.stateOf(len(rows), failure)
}

func (sidebar *SidebarService) stateOf(rowCount int, failure error) string {
	switch {
	case failure != nil:
		return "error"
	case rowCount == 0:
		return sidebar.stateEmpty()
	}
	return sidebar.stateReady()
}

func (sidebar *SidebarService) wantsRefresh(eventType string) bool {
	loaded := sidebar.State == sidebar.stateReady() || sidebar.State == sidebar.stateEmpty()
	return loaded && slices.Contains(sidebar.refreshTypes(), eventType)
}

func (sidebar *SidebarService) refreshWhen(wanted bool) {
	if !wanted {
		return
	}
	sidebar.refresh(sidebar.DomainId)
}

func (sidebar *SidebarService) rowOf(conversationId string) ConversationRow {
	found := ConversationRow{}
	for _, row := range slices.Concat(sidebar.Channels, sidebar.Directs, sidebar.Emails) {
		found = sidebar.matching(found, row, conversationId)
	}
	return found
}

func (sidebar *SidebarService) matching(found ConversationRow, row ConversationRow, conversationId string) ConversationRow {
	if row.ConversationId == conversationId {
		return row
	}
	return found
}

func (sidebar *SidebarService) openWhenKnown(row ConversationRow) {
	if row.ConversationId == "" {
		return
	}
	sidebar.open(row)
}

func (sidebar *SidebarService) open(row ConversationRow) {
	sidebar.CurrentId = row.ConversationId
	sidebar.Channels = sidebar.marked(sidebar.Channels, row.ConversationId)
	sidebar.Directs = sidebar.marked(sidebar.Directs, row.ConversationId)
	sidebar.Emails = sidebar.marked(sidebar.Emails, row.ConversationId)
	sidebar.shell.Pane = sidebar.paneMain()
	sidebar.shell.Heading = row.Title
	sidebar.applicationEventPublisher.PublishEvent(ConversationOpened{
		DomainId:       sidebar.DomainId,
		ConversationId: row.ConversationId,
		Title:          row.Title,
		Section:        row.Section,
		UnreadCount:    row.UnreadCount,
	}, true)
}

func (sidebar *SidebarService) marked(rows []ConversationRow, conversationId string) []ConversationRow {
	for index := range rows {
		rows[index] = sidebar.markedRow(rows[index], conversationId)
	}
	return rows
}

func (sidebar *SidebarService) markedRow(row ConversationRow, conversationId string) ConversationRow {
	row.Current = row.ConversationId == conversationId
	if !row.Current {
		return row
	}
	row.UnreadCount = 0
	row.MentionCount = 0
	row.Unread = false
	row.CountLabel = ""
	return row
}

func (sidebar *SidebarService) toggle(section string) {
	switch section {
	case sidebar.sectionChannels():
		sidebar.ChannelsClosed = !sidebar.ChannelsClosed
	case sidebar.sectionDirect():
		sidebar.DirectsClosed = !sidebar.DirectsClosed
	case sidebar.sectionEmail():
		sidebar.EmailsClosed = !sidebar.EmailsClosed
	}
}

func (sidebar *SidebarService) refreshTypes() []string {
	return []string{"message.created", "message.deleted", "read.moved"}
}

func (sidebar *SidebarService) pageSize() int {
	return 50
}

func (sidebar *SidebarService) paneMain() string {
	return "main"
}

func (sidebar *SidebarService) stateLoading() string {
	return "loading"
}

func (sidebar *SidebarService) stateReady() string {
	return "ready"
}

func (sidebar *SidebarService) stateEmpty() string {
	return "empty"
}

func (sidebar *SidebarService) sectionChannels() string {
	return "channels"
}

func (sidebar *SidebarService) sectionDirect() string {
	return "direct"
}

func (sidebar *SidebarService) sectionEmail() string {
	return "email"
}
