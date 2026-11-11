import "errors"

// @Test(ConversationTestSetup, {"partition":"tests"})
type SidebarServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SidebarServiceTest) sidebar(accessToken string) *SidebarService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "ana", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com"})
	tokens := &SessionTokenService{identity: NewIdentityClientWith(test.behaviors)}
	tokens.tokens = map[string]SessionTokens{"example.com": {AccessToken: accessToken, ExpiresAtMs: 100000000}}
	return &SidebarService{
		loader: &SidebarLoadService{
			messaging:      NewMessagingClientWith(test.behaviors),
			channelsClient: NewChannelsClientWith(test.behaviors),
			token:          &ConversationTokenService{session: &SessionService{tokens: tokens, accounts: accounts}, accounts: accounts},
		},
		shell: &ShellService{},
		rows:  &SidebarRowService{},
	}
}

func (test *SidebarServiceTest) loaded() *SidebarService {
	sidebar := test.sidebar("access-1")
	sidebar.DomainSelected(ConversationDomainSelected{DomainId: "example.com"})
	return sidebar
}

func (test *SidebarServiceTest) useToken(sidebar *SidebarService, accessToken string) {
	sidebar.loader.token.session.tokens.tokens["example.com"] = SessionTokens{AccessToken: accessToken, ExpiresAtMs: 100000000}
}

func (test *SidebarServiceTest) ids(rows []ConversationRow) []string {
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row.ConversationId)
	}
	return ids
}

func (test *SidebarServiceTest) TestDomainSelectedFillsTheThreeSections() {
	sidebar := test.loaded()
	Assert.Equals("ready", sidebar.State)
	Assert.Equals("example.com", sidebar.DomainId)
	Assert.Equals([]string{"cnv_general", "cnv_secret"}, test.ids(sidebar.Channels))
	Assert.Equals([]string{"cnv_team", "cnv_bo"}, test.ids(sidebar.Directs))
	Assert.Equals([]string{"cnv_carol"}, test.ids(sidebar.Emails))
}

func (test *SidebarServiceTest) TestDomainSelectedMatchesChannelInfo() {
	sidebar := test.loaded()
	Assert.IsTrue(sidebar.Channels[0].HasEmailAddress)
	Assert.IsTrue(sidebar.Channels[1].Private)
	Assert.IsTrue(sidebar.Channels[1].Archived)
}

func (test *SidebarServiceTest) TestDomainSelectedShowsAnErrorWhenConversationsFail() {
	sidebar := test.sidebar("access-2")
	sidebar.DomainSelected(ConversationDomainSelected{DomainId: "example.com"})
	Assert.Equals("error", sidebar.State)
	Assert.Equals(0, len(sidebar.Channels))
}

func (test *SidebarServiceTest) TestDomainSelectedShowsAnErrorWhenTheSessionEnded() {
	sidebar := test.sidebar("access-1")
	sidebar.loader.token.session.tokens.tokens["example.com"] = SessionTokens{AccessToken: "access-1", ExpiresAtMs: 0}
	sidebar.DomainSelected(ConversationDomainSelected{DomainId: "example.com"})
	Assert.Equals("error", sidebar.State)
}

func (test *SidebarServiceTest) TestDomainSelectedClearsTheRowsOfTheOldDomain() {
	sidebar := test.loaded()
	sidebar.Open("cnv_general")
	test.useToken(sidebar, "access-2")
	sidebar.DomainSelected(ConversationDomainSelected{DomainId: "example.org"})
	Assert.Equals("example.org", sidebar.DomainId)
	Assert.Equals("", sidebar.CurrentId)
	Assert.Equals(0, len(sidebar.Channels))
	Assert.Equals(0, len(sidebar.Directs))
	Assert.Equals(0, len(sidebar.Emails))
}

func (test *SidebarServiceTest) TestAnAnswerForAnotherDomainIsDropped() {
	sidebar := test.loaded()
	sidebar.applyWhenCurrent("example.org", []ConversationSummary{}, []Channel{}, nil)
	Assert.Equals(2, len(sidebar.Channels))
	Assert.Equals("ready", sidebar.State)
}

func (test *SidebarServiceTest) TestRowsStayWhenTheChannelListFails() {
	sidebar := test.sidebar("access-1")
	sidebar.DomainId = "example.com"
	channels := sidebar.loader.channelsOrNone(nil, errors.New("channels are down"))
	sidebar.applyWhenCurrent("example.com", ConversationTestSetup{}.Summaries(), channels, nil)
	Assert.Equals("ready", sidebar.State)
	Assert.Equals(2, len(sidebar.Channels))
	Assert.IsTrue(!sidebar.Channels[1].Private)
}

func (test *SidebarServiceTest) TestNoConversationsShowsEmpty() {
	sidebar := test.sidebar("access-1")
	sidebar.DomainId = "example.com"
	sidebar.applyWhenCurrent("example.com", []ConversationSummary{}, []Channel{}, nil)
	Assert.Equals("empty", sidebar.State)
}

func (test *SidebarServiceTest) TestOpenMarksTheRowCurrentAndRead() {
	sidebar := test.loaded()
	sidebar.Open("cnv_general")
	row := sidebar.Channels[0]
	Assert.Equals("cnv_general", sidebar.CurrentId)
	Assert.IsTrue(row.Current)
	Assert.IsTrue(!row.Unread)
	Assert.Equals(0, row.UnreadCount)
	Assert.Equals(0, row.MentionCount)
	Assert.Equals("", row.CountLabel)
}

func (test *SidebarServiceTest) TestOpenShowsTheConversationPane() {
	sidebar := test.loaded()
	sidebar.Open("cnv_carol")
	Assert.Equals("main", sidebar.shell.Pane)
	Assert.Equals("Carol Diaz", sidebar.shell.Heading)
}

func (test *SidebarServiceTest) TestOpenMovesCurrentFromThePreviousRow() {
	sidebar := test.loaded()
	sidebar.Open("cnv_general")
	sidebar.Open("cnv_bo")
	Assert.IsTrue(!sidebar.Channels[0].Current)
	Assert.IsTrue(sidebar.Directs[1].Current)
	Assert.Equals(0, sidebar.Directs[1].UnreadCount)
	Assert.Equals("cnv_bo", sidebar.CurrentId)
}

func (test *SidebarServiceTest) TestOpenOfAnUnknownConversationChangesNothing() {
	sidebar := test.loaded()
	sidebar.Open("cnv_missing")
	Assert.Equals("", sidebar.CurrentId)
	Assert.Equals("", sidebar.shell.Pane)
}

func (test *SidebarServiceTest) TestToggleSectionClosesAndOpensEachSection() {
	sidebar := test.loaded()
	sidebar.ToggleSection("channels")
	sidebar.ToggleSection("direct")
	sidebar.ToggleSection("email")
	Assert.IsTrue(sidebar.ChannelsClosed)
	Assert.IsTrue(sidebar.DirectsClosed)
	Assert.IsTrue(sidebar.EmailsClosed)
	sidebar.ToggleSection("channels")
	Assert.IsTrue(!sidebar.ChannelsClosed)
}

func (test *SidebarServiceTest) TestRetryLoadsTheSameDomainAgain() {
	sidebar := test.sidebar("access-2")
	sidebar.DomainSelected(ConversationDomainSelected{DomainId: "example.com"})
	test.useToken(sidebar, "access-1")
	sidebar.Retry()
	Assert.Equals("ready", sidebar.State)
	Assert.Equals("example.com", sidebar.DomainId)
	Assert.Equals(2, len(sidebar.Channels))
}

func (test *SidebarServiceTest) TestLiveRefreshesAndKeepsTheOpenConversation() {
	sidebar := test.loaded()
	sidebar.Open("cnv_general")
	sidebar.ToggleSection("direct")
	sidebar.Directs = []ConversationRow{}
	sidebar.Live(RealtimeEvent{Type: "message.created", ConversationId: "cnv_bo", MessageId: "m-7"})
	Assert.Equals(2, len(sidebar.Directs))
	Assert.Equals("cnv_general", sidebar.CurrentId)
	Assert.IsTrue(sidebar.Channels[0].Current)
	Assert.IsTrue(sidebar.DirectsClosed)
	Assert.IsTrue(sidebar.Channels[1].Private)
}

func (test *SidebarServiceTest) TestLiveRefreshesOnReadAndDelete() {
	sidebar := test.loaded()
	sidebar.Emails = []ConversationRow{}
	sidebar.Live(RealtimeEvent{Type: "read.moved", ConversationId: "cnv_carol"})
	Assert.Equals(1, len(sidebar.Emails))
	sidebar.Emails = []ConversationRow{}
	sidebar.Live(RealtimeEvent{Type: "message.deleted", ConversationId: "cnv_carol"})
	Assert.Equals(1, len(sidebar.Emails))
}

func (test *SidebarServiceTest) TestLiveIgnoresOtherEvents() {
	sidebar := test.loaded()
	sidebar.Directs = []ConversationRow{}
	sidebar.Live(RealtimeEvent{Type: "auth.accepted"})
	Assert.Equals(0, len(sidebar.Directs))
}

func (test *SidebarServiceTest) TestLiveWaitsUntilTheSidebarIsLoaded() {
	sidebar := test.sidebar("access-1")
	sidebar.Live(RealtimeEvent{Type: "message.created", ConversationId: "cnv_bo"})
	Assert.Equals(0, len(sidebar.Directs))
	Assert.Equals("", sidebar.State)
}

func (test *SidebarServiceTest) TestLiveKeepsTheRowsWhenTheRefreshFails() {
	sidebar := test.loaded()
	test.useToken(sidebar, "access-2")
	sidebar.Live(RealtimeEvent{Type: "message.created", ConversationId: "cnv_bo"})
	Assert.Equals("ready", sidebar.State)
	Assert.Equals(2, len(sidebar.Directs))
}

func (test *SidebarServiceTest) TestStartShowsLoadingUntilTheDomainLoads() {
	sidebar := test.sidebar("access-1")
	sidebar.Start()
	Assert.Equals("loading", sidebar.State)
}

func (test *SidebarServiceTest) TestDomainChangedLoadsTheNewDomain() {
	sidebar := test.sidebar("access-1")
	sidebar.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	Assert.Equals("ready", sidebar.State)
	Assert.Equals("example.com", sidebar.DomainId)
}

func (test *SidebarServiceTest) TestViewMountedAnswersTheOpenConversation() {
	sidebar := test.loaded()
	sidebar.Open("cnv_general")
	opened := sidebar.ViewMounted(ConversationViewMounted{Mounted: true})
	Assert.Equals("cnv_general", opened.ConversationId)
	Assert.Equals("example.com", opened.DomainId)
	Assert.Equals(2, opened.UnreadCount)
}

func (test *SidebarServiceTest) TestViewMountedBeforeAnyOpenAnswersNothing() {
	opened := test.loaded().ViewMounted(ConversationViewMounted{Mounted: true})
	Assert.Equals("", opened.ConversationId)
}

func (test *SidebarServiceTest) TestNewMessageClearsTheCurrentConversation() {
	sidebar := test.loaded()
	sidebar.Open("cnv_general")
	sidebar.NewMessage()
	Assert.Equals("", sidebar.CurrentId)
	Assert.IsTrue(!sidebar.Channels[0].Current)
	Assert.Equals("main", sidebar.shell.Pane)
	Assert.Equals("New message", sidebar.shell.Heading)
	Assert.Equals("", sidebar.ViewMounted(ConversationViewMounted{Mounted: true}).ConversationId)
}

func (test *SidebarServiceTest) TestRefreshedCountsRenders() {
	sidebar := test.loaded()
	sidebar.Refreshed(ConversationRefreshed{Source: "conversation"})
	Assert.Equals(1, sidebar.refreshes)
}
