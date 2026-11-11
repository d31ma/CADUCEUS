// @Test(ChannelTestSetup, {"partition":"tests"})
type ChannelArchiveServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	channelsClient *ChannelsClient
	// @Fake
	identityClient *IdentityClient
}

func (test *ChannelArchiveServiceTest) archive() *ChannelArchiveService {
	session := &AdminSessionService{}
	test.construct(session)
	list := &ChannelListService{session: session, problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "channels", DomainId: "example.com"})
	archive := &ChannelArchiveService{channelList: list, session: session, problems: &AdminProblemService{}}
	test.construct(archive)
	return archive
}

func (test *ChannelArchiveServiceTest) asked(channelId string) *ChannelArchiveService {
	archive := test.archive()
	archive.Ask(channelId)
	return archive
}

func (test *ChannelArchiveServiceTest) TestAskOpensTheConfirmStep() {
	archive := test.asked("general-id")
	Assert.Equals(true, archive.Open)
	Assert.Equals("general-id", archive.ChannelId)
	Assert.Equals("Archive #general?", archive.Title)
	Assert.Equals("#general has no email address. When a channel with an address is archived, mail to that address is refused.", archive.AddressNote)
}

func (test *ChannelArchiveServiceTest) TestAskNamesTheAddressThatStopsReceivingMail() {
	archive := test.asked("support-id")
	Assert.Equals("Mail to support@example.com is refused while #support is archived.", archive.AddressNote)
}

func (test *ChannelArchiveServiceTest) TestCancelKeepsTheChannel() {
	archive := test.asked("general-id")
	archive.Cancel()
	Assert.Equals(false, archive.Open)
	Assert.Equals(false, archive.channelList.Rows[0].Archived)
}

func (test *ChannelArchiveServiceTest) TestConfirmArchivesTheChannel() {
	archive := test.asked("general-id")
	archive.Confirm()
	Assert.Equals("", archive.Problem)
	Assert.Equals(false, archive.Open)
	Assert.Equals(false, archive.Busy)
	Assert.Equals(true, archive.channelList.Rows[0].Archived)
	Assert.Equals("is-muted", archive.channelList.Rows[0].RowClass)
}

func (test *ChannelArchiveServiceTest) TestConfirmUpdatesTheArchivedCount() {
	archive := test.asked("general-id")
	archive.Confirm()
	Assert.Equals("2 with an email address · 2 archived", archive.channelList.Summary)
}

func (test *ChannelArchiveServiceTest) TestConfirmSaysOnlyADomainAdminMayArchive() {
	archive := test.asked("locked-id")
	archive.Confirm()
	Assert.Equals("Only a domain admin can archive channels.", archive.Problem)
	Assert.Equals(true, archive.Open)
}

func (test *ChannelArchiveServiceTest) TestConfirmSaysWhenTheChannelIsGone() {
	archive := test.asked("gone-id")
	archive.Confirm()
	Assert.Equals("This channel no longer exists.", archive.Problem)
	Assert.Equals(true, archive.Open)
}

func (test *ChannelArchiveServiceTest) TestRestoreOpensTheChannelAgain() {
	archive := test.archive()
	archive.Restore("launch-id")
	Assert.Equals("", archive.Notice)
	Assert.Equals(false, archive.channelList.Rows[3].Archived)
	Assert.Equals("", archive.channelList.Rows[3].RowClass)
}

func (test *ChannelArchiveServiceTest) TestRestoreSaysWhyTheChannelIsStillArchived() {
	archive := test.archive()
	archive.Restore("gone-id")
	Assert.Equals("#old-news is still archived. This channel no longer exists.", archive.Notice)
	Assert.Equals(false, archive.Open)
}
