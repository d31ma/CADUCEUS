// @Test(ChannelEmptyTestSetup, {"partition":"tests"})
type ChannelListEmptyTest struct {
	behaviors *BehaviorContext
	// @Fake
	channelsClient *ChannelsClient
	// @Fake
	identityClient *IdentityClient
}

func (test *ChannelListEmptyTest) TestOpenedWithoutChannelsShowsTheEmptyState() {
	session := &AdminSessionService{}
	test.construct(session)
	list := &ChannelListService{session: session, problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "channels", DomainId: "example.com"})
	Assert.Equals("empty", list.State)
	Assert.Equals("0 channels", list.Heading)
	Assert.Equals(0, len(list.Rows))
}
