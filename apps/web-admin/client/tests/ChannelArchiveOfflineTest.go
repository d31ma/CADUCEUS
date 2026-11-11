// @Test(ChannelOfflineTestSetup, {"partition":"tests"})
type ChannelArchiveOfflineTest struct {
	behaviors *BehaviorContext
	// @Fake
	channelsClient *ChannelsClient
	// @Fake
	identityClient *IdentityClient
}

func (test *ChannelArchiveOfflineTest) TestConfirmSaysWhenTheServerIsUnreachable() {
	session := &AdminSessionService{}
	test.construct(session)
	general := Channel{ChannelId: "general-id", Name: "general", Visibility: "public"}
	list := &ChannelListService{channels: []Channel{general}, session: session, problems: &AdminProblemService{}}
	test.construct(list)
	archive := &ChannelArchiveService{channelList: list, session: session, problems: &AdminProblemService{}}
	test.construct(archive)
	archive.Ask("general-id")
	archive.Confirm()
	Assert.Equals("Mesail could not reach the server. Check your connection, then try again.", archive.Problem)
	Assert.Equals(true, archive.Open)
}
