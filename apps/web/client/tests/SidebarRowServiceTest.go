// @Test(ConversationTestSetup, {"partition":"tests"})
type SidebarRowServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SidebarRowServiceTest) rows() []ConversationRow {
	setup := ConversationTestSetup{}
	return (&SidebarRowService{}).Rows(setup.Summaries(), setup.Channels(), "cnv_bo")
}

func (test *SidebarRowServiceTest) section(section string) []ConversationRow {
	return (&SidebarRowService{}).Section(test.rows(), section)
}

func (test *SidebarRowServiceTest) single(summary ConversationSummary) ConversationRow {
	return (&SidebarRowService{}).Rows([]ConversationSummary{summary}, []Channel{}, "")[0]
}

func (test *SidebarRowServiceTest) ids(rows []ConversationRow) []string {
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row.ConversationId)
	}
	return ids
}

func (test *SidebarRowServiceTest) TestRowsKeepTheServerOrder() {
	Assert.Equals([]string{"cnv_secret", "cnv_general", "cnv_bo", "cnv_team", "cnv_carol"}, test.ids(test.rows()))
}

func (test *SidebarRowServiceTest) TestRowsPutEachConversationInItsSection() {
	rows := test.rows()
	Assert.Equals("channels", rows[0].Section)
	Assert.Equals("direct", rows[2].Section)
	Assert.Equals("email", rows[4].Section)
}

func (test *SidebarRowServiceTest) TestSectionSortsChannelsByTitle() {
	Assert.Equals([]string{"cnv_general", "cnv_secret"}, test.ids(test.section("channels")))
}

func (test *SidebarRowServiceTest) TestSectionSortsDirectNewestFirst() {
	Assert.Equals([]string{"cnv_team", "cnv_bo"}, test.ids(test.section("direct")))
}

func (test *SidebarRowServiceTest) TestSectionHoldsExternalConversationsInEmail() {
	Assert.Equals([]string{"cnv_carol"}, test.ids(test.section("email")))
}

func (test *SidebarRowServiceTest) TestChannelInitialsUseTheFirstTwoLetters() {
	Assert.Equals("GE", test.rows()[1].Initials)
}

func (test *SidebarRowServiceTest) TestPersonInitialsUseTheFirstTwoWords() {
	rows := test.rows()
	Assert.Equals("BL", rows[2].Initials)
	Assert.Equals("BL", rows[3].Initials)
	Assert.Equals("CD", rows[4].Initials)
}

func (test *SidebarRowServiceTest) TestOneWordInitialsUseTwoLetters() {
	Assert.Equals("CA", test.single(ConversationSummary{ConversationId: "cnv_x", Kind: "direct", Title: "carol"}).Initials)
}

func (test *SidebarRowServiceTest) TestEmptyTitleHasNoInitials() {
	Assert.Equals("", test.single(ConversationSummary{ConversationId: "cnv_x", Kind: "direct"}).Initials)
}

func (test *SidebarRowServiceTest) TestCountLabelNamesUnreadAndMentions() {
	row := test.rows()[1]
	Assert.Equals("2 unread, 1 mention", row.CountLabel)
	Assert.IsTrue(row.Unread)
}

func (test *SidebarRowServiceTest) TestCountLabelWithOnlyUnread() {
	Assert.Equals("1 unread", test.rows()[2].CountLabel)
}

func (test *SidebarRowServiceTest) TestCountLabelIsEmptyWhenAllIsRead() {
	row := test.rows()[3]
	Assert.Equals("", row.CountLabel)
	Assert.IsTrue(!row.Unread)
}

func (test *SidebarRowServiceTest) TestCountLabelCountsSeveralMentions() {
	row := test.single(ConversationSummary{ConversationId: "cnv_x", Kind: "channel", Title: "x", UnreadCount: 3, MentionCount: 2})
	Assert.Equals("3 unread, 2 mentions", row.CountLabel)
}

func (test *SidebarRowServiceTest) TestCountLabelWithOnlyAMention() {
	row := test.single(ConversationSummary{ConversationId: "cnv_x", Kind: "channel", Title: "x", MentionCount: 1})
	Assert.Equals("1 mention", row.CountLabel)
}

func (test *SidebarRowServiceTest) TestChannelInfoComesFromTheMatchingChannel() {
	rows := test.rows()
	Assert.IsTrue(rows[0].Private)
	Assert.IsTrue(rows[0].Archived)
	Assert.IsTrue(!rows[0].HasEmailAddress)
	Assert.IsTrue(!rows[1].Private)
	Assert.IsTrue(rows[1].HasEmailAddress)
}

func (test *SidebarRowServiceTest) TestOnlyTheCurrentConversationIsCurrent() {
	rows := test.rows()
	Assert.IsTrue(rows[2].Current)
	Assert.IsTrue(!rows[1].Current)
	Assert.Equals("page", *rows[2].AriaCurrent)
	Assert.IsTrue(rows[1].AriaCurrent == nil)
}

func (test *SidebarRowServiceTest) TestAriaCurrentIsPageOnlyForTheCurrentRow() {
	service := &SidebarRowService{}
	Assert.Equals("page", *service.AriaCurrent(true))
	Assert.IsTrue(service.AriaCurrent(false) == nil)
}
