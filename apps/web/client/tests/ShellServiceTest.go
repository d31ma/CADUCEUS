// @Test(ShellTestSetup, {"partition":"tests"})
type ShellServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ShellServiceTest) TestStartNamesTheProduct() {
	shell := &ShellService{}
	shell.Start()
	Assert.Equals("Mesail", shell.ProductName)
}

func (test *ShellServiceTest) TestStartAsksForAConversation() {
	shell := &ShellService{}
	shell.Start()
	Assert.Equals("Select a conversation", shell.Heading)
}
