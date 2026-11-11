// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainRemoveServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainRemoveServiceTest) started(domainId string) *DomainRemoveService {
	session := &AdminSessionService{}
	test.construct(session)
	shell := &AdminShellService{}
	test.construct(shell)
	shell.Open("domain", domainId)
	remove := &DomainRemoveService{session: session, problems: &AdminProblemService{}, adminShell: shell}
	test.construct(remove)
	remove.Start(domainId, domainId)
	return remove
}

func (test *DomainRemoveServiceTest) TestStartAsksToTypeTheName() {
	remove := test.started("example.com")
	Assert.Equals(true, remove.Confirming)
	Assert.Equals("example.com", remove.Name)
	Assert.Equals(false, remove.CanRemove)
	Assert.Equals(true, remove.Locked)
}

func (test *DomainRemoveServiceTest) TestTypeTheRightNameAllowsRemove() {
	remove := test.started("example.com")
	remove.Type("example.com")
	Assert.Equals(true, remove.CanRemove)
	Assert.Equals(false, remove.Locked)
}

func (test *DomainRemoveServiceTest) TestTypeAnotherNameDoesNotAllowRemove() {
	remove := test.started("example.com")
	remove.Type("example.co")
	Assert.Equals(false, remove.CanRemove)
}

func (test *DomainRemoveServiceTest) TestCancelClosesTheConfirm() {
	remove := test.started("example.com")
	remove.Type("example.com")
	remove.Cancel()
	Assert.Equals(false, remove.Confirming)
	Assert.Equals("", remove.Typed)
	Assert.Equals(false, remove.CanRemove)
}

func (test *DomainRemoveServiceTest) TestOpenedClosesAnOldConfirm() {
	remove := test.started("example.com")
	remove.Opened(AdminViewOpened{View: "domain", DomainId: "example.org"})
	Assert.Equals(false, remove.Confirming)
}

func (test *DomainRemoveServiceTest) TestRemoveOpensTheDomainList() {
	remove := test.started("example.com")
	remove.Type("example.com")
	remove.Remove()
	Assert.Equals("", remove.Problem)
	Assert.Equals(false, remove.Confirming)
	Assert.Equals("domains", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestRemoveWithoutTheNameDoesNothing() {
	remove := test.started("example.com")
	remove.Remove()
	Assert.Equals(true, remove.Confirming)
	Assert.Equals("domain", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestRemoveWithoutOperatorRightsSaysSo() {
	remove := test.started("old.example.com")
	remove.Type("old.example.com")
	remove.Remove()
	Assert.Equals("Only operators can remove domains.", remove.Problem)
	Assert.Equals(true, remove.Confirming)
	Assert.Equals("domain", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestRemoveOfAGoneDomainSaysSo() {
	remove := test.started("gone.example")
	remove.Type("gone.example")
	remove.Remove()
	Assert.Equals("This domain was already removed.", remove.Problem)
}

func (test *DomainRemoveServiceTest) TestRemoveWithoutTheServerSaysSo() {
	remove := test.started("new.example.net")
	remove.Type("new.example.net")
	remove.Remove()
	Assert.Equals("The domain was not removed. Mesail could not reach the server. Try again.", remove.Problem)
	Assert.Equals(false, remove.Removing)
}
