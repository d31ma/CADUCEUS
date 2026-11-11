// @Test(ShellTestSetup, {"partition":"tests"})
type ShellServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ShellServiceTest) shell() *ShellService {
	shell := &ShellService{}
	shell.Start()
	return shell
}

func (test *ShellServiceTest) TestStartNamesTheProduct() {
	Assert.Equals("Mesail", test.shell().ProductName)
}

func (test *ShellServiceTest) TestStartAsksForAConversation() {
	Assert.Equals("Select a conversation", test.shell().Heading)
}

func (test *ShellServiceTest) TestStartShowsTheList() {
	Assert.Equals("list", test.shell().Pane)
}

func (test *ShellServiceTest) TestShowListClosesTheMenus() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	shell.ShowList()
	Assert.Equals("list", shell.Pane)
	Assert.IsTrue(!shell.DomainMenuOpen)
}

func (test *ShellServiceTest) TestToggleDomainMenuOpensAndCloses() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	Assert.IsTrue(shell.DomainMenuOpen)
	shell.ToggleDomainMenu()
	Assert.IsTrue(!shell.DomainMenuOpen)
}

func (test *ShellServiceTest) TestCloseThreadReturnsToTheConversation() {
	shell := test.shell()
	shell.ThreadOpen = true
	shell.CloseThread()
	Assert.Equals("main", shell.Pane)
	Assert.IsTrue(!shell.ThreadOpen)
}

func (test *ShellServiceTest) TestCloseMenusClosesTheDomainMenu() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	shell.CloseMenus()
	Assert.IsTrue(!shell.DomainMenuOpen)
}
