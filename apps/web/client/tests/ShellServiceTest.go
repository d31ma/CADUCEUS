// @Test(SessionTestSetup, {"partition":"tests"})
type ShellServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ShellServiceTest) shell() *ShellService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	client := NewIdentityClientWith(test.behaviors)
	accounts := &SessionAccountService{}
	session := &SessionService{tokens: &SessionTokenService{identity: client}, accounts: accounts, problems: &SessionProblemService{}}
	session.SignInWith("ana", "right", "", false)
	session.SignIn(SessionSignInRequested{})
	shell := &ShellService{accounts: accounts, settings: &SessionSettingsService{session: session, identity: client, problems: &SessionProblemService{}}}
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

func (test *ShellServiceTest) TestFlagWritesTrueForScreenReaders() {
	Assert.Equals("true", test.shell().Flag(true))
}

func (test *ShellServiceTest) TestFlagWritesFalseForScreenReaders() {
	Assert.Equals("false", test.shell().Flag(false))
}

func (test *ShellServiceTest) TestFieldTypeShowsThePassword() {
	Assert.Equals("text", test.shell().FieldType(true))
}

func (test *ShellServiceTest) TestFieldTypeHidesThePassword() {
	Assert.Equals("password", test.shell().FieldType(false))
}

func (test *ShellServiceTest) TestShowPageOpensSettingsInTheMainPane() {
	shell := test.shell()
	shell.ShowPage("settings")
	Assert.Equals("on", shell.settings.TwoStep)
	Assert.Equals("settings", shell.Page)
	Assert.Equals("main", shell.Pane)
	Assert.Equals("Settings", shell.Title())
}

func (test *ShellServiceTest) TestShowPageHomeShowsTheList() {
	shell := test.shell()
	shell.ShowPage("activity")
	shell.ShowPage("")
	Assert.Equals("list", shell.Pane)
	Assert.Equals("Select a conversation", shell.Title())
}

func (test *ShellServiceTest) TestShowListLeavesTheOtherPages() {
	shell := test.shell()
	shell.ShowPage("settings")
	shell.ShowList()
	Assert.Equals("", shell.Page)
}

func (test *ShellServiceTest) TestRestoredMarksTheShellReady() {
	shell := test.shell()
	shell.Restored(SessionRestored{SignedIn: true})
	Assert.IsTrue(shell.Ready)
}

func (test *ShellServiceTest) TestCloseMenusMarksNewDomainsSeen() {
	shell := test.shell()
	shell.accounts.Apply(Session{Me: Me{DomainId: "example.com", Username: "ana"}, Domains: []DomainChoice{{DomainId: "example.com"}, {DomainId: "partner.test"}}})
	Assert.IsTrue(shell.accounts.Accounts[1].New)
	shell.CloseMenus()
	Assert.Equals(false, shell.accounts.Accounts[1].New)
}

func (test *ShellServiceTest) TestSessionChangedCountsTheChange() {
	shell := test.shell()
	shell.SessionChanged(SessionChanged{})
	Assert.Equals(1, shell.SessionChanges)
}

func (test *ShellServiceTest) TestTwoStepLoadedCountsTheChange() {
	shell := test.shell()
	shell.TwoStepLoaded(SessionTwoStepLoaded{})
	Assert.Equals(1, shell.SessionChanges)
}

func (test *ShellServiceTest) TestDomainChangedClosesTheMenus() {
	shell := test.shell()
	shell.ToggleDomainMenu()
	shell.DomainChanged(SessionDomainChanged{DomainId: "example.org"})
	Assert.Equals(false, shell.DomainMenuOpen)
}
