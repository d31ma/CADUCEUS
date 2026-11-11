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
	opened         ConversationOpened
	Refreshes      int
	// @Needs
	loader *SidebarLoadService
	// @Needs
	shell *ShellService
	// @Needs
	rows                      *SidebarRowService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (sidebar *SidebarService) Start() {
	sidebar.State = sidebar.stateLoading()
	sidebar.applicationEventPublisher.PublishEvent(ConversationDomainSelected{DomainId: sidebar.loader.DomainId()}, false)
}

// @OnEvent({"replay":true})
// @InBackground
func (sidebar *SidebarService) DomainSelected(selected ConversationDomainSelected) {
	sidebar.clear(selected.DomainId)
	sidebar.load(selected.DomainId)
	sidebar.announceRefresh()
}

// @OnEvent({"replay":true})
// @InBackground
func (sidebar *SidebarService) DomainChanged(changed SessionDomainChanged) {
	sidebar.resetShellWhenOtherDomain(changed.DomainId)
	sidebar.clear(changed.DomainId)
	sidebar.load(changed.DomainId)
	sidebar.announceRefresh()
}

// @OnEvent
func (sidebar *SidebarService) Refreshed(refreshed ConversationRefreshed) {
	sidebar.Refreshes++
}

// @OnEvent
// @InBackground
func (sidebar *SidebarService) Sent(completed SendCompleted) {
	sidebar.refresh(sidebar.DomainId)
	sidebar.Open(sidebar.newConversationId(completed.ConversationId))
	sidebar.announceRefresh()
}

// @OnEvent
func (sidebar *SidebarService) ViewMounted(mounted ConversationViewMounted) ConversationOpened {
	return sidebar.opened
}

// @OnEvent
// @InBackground
func (sidebar *SidebarService) Live(event RealtimeEvent) {
	sidebar.refreshWhen(sidebar.wantsRefresh(event.Type))
	sidebar.announceRefresh()
}

func (sidebar *SidebarService) Open(conversationId string) {
	sidebar.openWhenKnown(sidebar.rowOf(conversationId))
}

func (sidebar *SidebarService) NewMessage() {
	sidebar.markCurrent("")
	sidebar.opened = ConversationOpened{}
	sidebar.shell.Pane = sidebar.paneMain()
	sidebar.shell.Heading = sidebar.newMessageHeading()
	sidebar.applicationEventPublisher.PublishEvent(SendTarget{}, true)
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
	sidebar.opened = ConversationOpened{}
}

func (sidebar *SidebarService) resetShellWhenOtherDomain(domainId string) {
	if domainId == sidebar.DomainId {
		return
	}
	sidebar.shell.ShowList()
	sidebar.shell.Heading = sidebar.listHeading()
}

func (sidebar *SidebarService) newConversationId(conversationId string) string {
	if sidebar.CurrentId != "" {
		return ""
	}
	return conversationId
}

func (sidebar *SidebarService) listHeading() string {
	return "Select a conversation"
}

func (sidebar *SidebarService) stateLoading() string {
	return "loading"
}

func (sidebar *SidebarService) load(domainId string) {
	channels := sidebar.loader.Channels()
	summaries, failure := sidebar.loader.Summaries()
	sidebar.applyWhenCurrent(domainId, summaries, channels, failure)
}

func (sidebar *SidebarService) refresh(domainId string) {
	summaries, failure := sidebar.loader.Summaries()
	sidebar.applyLive(domainId, summaries, failure)
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

func (sidebar *SidebarService) stateReady() string {
	return "ready"
}

func (sidebar *SidebarService) stateEmpty() string {
	return "empty"
}

func (sidebar *SidebarService) wantsRefresh(eventType string) bool {
	return sidebar.allTrue(sidebar.isLoaded(), slices.Contains(sidebar.refreshTypes(), eventType))
}

func (sidebar *SidebarService) isLoaded() bool {
	return slices.Contains([]string{sidebar.stateReady(), sidebar.stateEmpty()}, sidebar.State)
}

func (sidebar *SidebarService) refreshTypes() []string {
	return []string{"message.created", "message.deleted", "read.moved"}
}

func (sidebar *SidebarService) allTrue(conditions ...bool) bool {
	return !slices.Contains(conditions, false)
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
	sidebar.markCurrent(row.ConversationId)
	sidebar.shell.Pane = sidebar.paneMain()
	sidebar.shell.Heading = row.Title
	sidebar.opened = ConversationOpened{
		DomainId:       sidebar.DomainId,
		ConversationId: row.ConversationId,
		Title:          row.Title,
		Section:        row.Section,
		UnreadCount:    row.UnreadCount,
	}
	sidebar.applicationEventPublisher.PublishEvent(sidebar.opened, false)
}

func (sidebar *SidebarService) markCurrent(conversationId string) {
	sidebar.CurrentId = conversationId
	sidebar.Channels = sidebar.marked(sidebar.Channels, conversationId)
	sidebar.Directs = sidebar.marked(sidebar.Directs, conversationId)
	sidebar.Emails = sidebar.marked(sidebar.Emails, conversationId)
}

func (sidebar *SidebarService) newMessageHeading() string {
	return "New message"
}

func (sidebar *SidebarService) paneMain() string {
	return "main"
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

func (sidebar *SidebarService) sectionChannels() string {
	return "channels"
}

func (sidebar *SidebarService) sectionDirect() string {
	return "direct"
}

func (sidebar *SidebarService) sectionEmail() string {
	return "email"
}

func (sidebar *SidebarService) announceRefresh() {
	sidebar.applicationEventPublisher.PublishEvent(ConversationRefreshed{Source: "sidebar"}, true)
}
