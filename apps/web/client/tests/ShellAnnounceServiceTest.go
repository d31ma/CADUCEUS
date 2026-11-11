// @Test(ShellTestSetup, {"partition":"tests"})
type ShellAnnounceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ShellAnnounceServiceTest) TestAnnounceSetsTheMessage() {
	announcer := &ShellAnnounceService{}
	announcer.Announce("New message from Bo Lee in #releases")
	Assert.Equals("New message from Bo Lee in #releases", announcer.Message)
}
