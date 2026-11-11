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

func (test *ShellServiceTest) TestShowMainShowsTheConversation() {
	shell := test.shell()
	shell.ShowMain()
	Assert.Equals("main", shell.Pane)
}

func (test *ShellServiceTest) TestShowListClosesTheMenus() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	shell.ShowList()
	Assert.Equals("list", shell.Pane)
	Assert.IsTrue(!shell.DomainMenuOpen)
}

func (test *ShellServiceTest) TestOpenThreadShowsTheThread() {
	shell := test.shell()
	shell.OpenThread()
	Assert.Equals("thread", shell.Pane)
	Assert.IsTrue(shell.ThreadOpen)
}

func (test *ShellServiceTest) TestCloseThreadReturnsToTheConversation() {
	shell := test.shell()
	shell.OpenThread()
	shell.CloseThread()
	Assert.Equals("main", shell.Pane)
	Assert.IsTrue(!shell.ThreadOpen)
}

func (test *ShellServiceTest) TestToggleDomainMenuOpensAndCloses() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	Assert.IsTrue(shell.DomainMenuOpen)
	shell.ToggleDomainMenu()
	Assert.IsTrue(!shell.DomainMenuOpen)
}

func (test *ShellServiceTest) TestToggleAccountMenuClosesTheDomainMenu() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	shell.ToggleAccountMenu()
	Assert.IsTrue(shell.AccountMenuOpen)
	Assert.IsTrue(!shell.DomainMenuOpen)
}

func (test *ShellServiceTest) TestCloseMenusClosesBoth() {
	shell := test.shell()
	shell.ToggleAccountMenu()
	shell.CloseMenus()
	Assert.IsTrue(!shell.AccountMenuOpen)
}
