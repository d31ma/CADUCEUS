// @Test(ChannelOfflineTestSetup, {"partition":"tests"})
type ChannelListOfflineTest struct {
	behaviors *BehaviorContext
	// @Fake
	channelsClient *ChannelsClient
	// @Fake
	identityClient *IdentityClient
}

func (test *ChannelListOfflineTest) TestOpenedWithoutTheServerShowsTheError() {
	session := &AdminSessionService{}
	test.construct(session)
	list := &ChannelListService{session: session, problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "channels", DomainId: "example.com"})
	Assert.Equals("failed", list.State)
	Assert.Equals("Mesail could not reach the server. Check your connection, then try again.", list.LoadProblem)
	Assert.Equals(0, len(list.Rows))
}
