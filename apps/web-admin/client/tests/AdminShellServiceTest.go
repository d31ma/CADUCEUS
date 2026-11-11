// @Test(AdminShellTestSetup, {"partition":"tests"})
type AdminShellServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AdminShellServiceTest) TestStartNamesThePage() {
	adminShell := &AdminShellService{}
	adminShell.Start()
	Assert.Equals("Mesail administration", adminShell.Heading)
}

func (test *AdminShellServiceTest) TestStartSaysWhatTheAdminManages() {
	adminShell := &AdminShellService{}
	adminShell.Start()
	Assert.Equals("Domains, channels and members", adminShell.Summary)
}
