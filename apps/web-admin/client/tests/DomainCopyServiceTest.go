// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainCopyServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
}

func (test *DomainCopyServiceTest) TestCopyRemembersTheCopiedText() {
	copying := &DomainCopyService{}
	copying.Copy("_dmarc.example.com")
	Assert.Equals("_dmarc.example.com", copying.Copied)
	Assert.Equals("Copied _dmarc.example.com.", copying.Notice)
	Assert.Equals("", copying.Problem)
}

func (test *DomainCopyServiceTest) TestOpenedForgetsTheCopiedText() {
	copying := &DomainCopyService{}
	copying.Copy("example.com")
	copying.Opened(AdminViewOpened{View: "domain", DomainId: "example.org"})
	Assert.Equals("", copying.Copied)
	Assert.Equals("", copying.Notice)
}
