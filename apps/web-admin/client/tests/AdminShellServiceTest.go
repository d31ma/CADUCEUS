// @Test(AdminShellTestSetup, {"partition":"tests"})
type AdminShellServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	identityClient *IdentityClient
}

func (test *AdminShellServiceTest) TestStartShowsTheMenu() {
	shell := &AdminShellService{}
	shell.Start()
	Assert.Equals("list", shell.Pane)
	Assert.Equals("", shell.View)
}

func (test *AdminShellServiceTest) TestAnOperatorStartsAtDomains() {
	shell := &AdminShellService{}
	test.construct(shell)
	shell.SignedIn(AdminSignedIn{DomainId: "mesail-operators", Operator: true})
	Assert.Equals("domains", shell.View)
	Assert.Equals("main", shell.Pane)
}

func (test *AdminShellServiceTest) TestADomainAdminStartsAtMembersOfTheirDomain() {
	shell := &AdminShellService{}
	test.construct(shell)
	shell.SignedIn(AdminSignedIn{DomainId: "example.com", DomainAdmin: true})
	Assert.Equals("members", shell.View)
	Assert.Equals("example.com", shell.DomainId)
}

func (test *AdminShellServiceTest) TestOpenShowsTheViewForTheDomain() {
	shell := &AdminShellService{}
	test.construct(shell)
	shell.Open("domain", "example.org")
	Assert.Equals("domain", shell.View)
	Assert.Equals("example.org", shell.DomainId)
}

func (test *AdminShellServiceTest) TestShowMenuGoesBackToTheList() {
	shell := &AdminShellService{}
	test.construct(shell)
	shell.Open("channels", "example.com")
	shell.ShowMenu()
	Assert.Equals("list", shell.Pane)
}

func (test *AdminShellServiceTest) TestFlagWritesAriaValuesAsText() {
	shell := &AdminShellService{}
	Assert.Equals("true", shell.Flag(true))
	Assert.Equals("false", shell.Flag(false))
}

func (test *AdminShellServiceTest) TestLoadedAnnouncesWhatTheViewShows() {
	shell := &AdminShellService{}
	shell.Loaded(AdminViewLoaded{View: "members", Message: "5 members"})
	shell.Loaded(AdminViewLoaded{View: "members", Message: ""})
	Assert.Equals("5 members", shell.Announcement)
}

func (test *AdminShellServiceTest) TestSidebarLabelNamesTheDomain() {
	shell := &AdminShellService{}
	Assert.Equals("Admin of example.com", shell.SidebarLabel("example.com"))
}
