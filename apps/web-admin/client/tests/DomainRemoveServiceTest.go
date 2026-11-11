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
	Assert.Equals("Remove example.com?", remove.Heading)
	Assert.Equals("Type example.com to confirm", remove.ConfirmLabel)
	Assert.Equals("", remove.Typed)
	Assert.Equals("", remove.NameProblem)
}

func (test *DomainRemoveServiceTest) TestConfirmWithAnotherNameDoesNotRemove() {
	remove := test.started("example.com")
	remove.ConfirmWith("example.co")
	Assert.Equals("The name does not match. Type example.com exactly to remove it.", remove.NameProblem)
	Assert.Equals("example.co", remove.Typed)
	Assert.Equals(true, remove.Confirming)
	Assert.Equals("domain", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestConfirmWithNothingDoesNotRemove() {
	remove := test.started("example.com")
	remove.ConfirmWith("")
	Assert.Equals(true, remove.Confirming)
	Assert.Equals("", remove.Problem)
	Assert.Equals("domain", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestConfirmWithTheNameOpensTheDomainList() {
	remove := test.started("example.com")
	remove.ConfirmWith(" example.com ")
	Assert.Equals("", remove.Problem)
	Assert.Equals(false, remove.Confirming)
	Assert.Equals("domains", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestCancelClosesTheConfirm() {
	remove := test.started("example.com")
	remove.ConfirmWith("example.co")
	remove.Cancel()
	Assert.Equals(false, remove.Confirming)
	Assert.Equals("", remove.Typed)
	Assert.Equals("", remove.NameProblem)
}

func (test *DomainRemoveServiceTest) TestOpenedClosesAnOldConfirm() {
	remove := test.started("example.com")
	remove.Opened(AdminViewOpened{View: "domain", DomainId: "example.org"})
	Assert.Equals(false, remove.Confirming)
}

func (test *DomainRemoveServiceTest) TestConfirmWithoutOperatorRightsSaysSo() {
	remove := test.started("old.example.com")
	remove.ConfirmWith("old.example.com")
	Assert.Equals("Only operators can remove domains.", remove.Problem)
	Assert.Equals("", remove.NameProblem)
	Assert.Equals(true, remove.Confirming)
	Assert.Equals("domain", remove.adminShell.View)
}

func (test *DomainRemoveServiceTest) TestConfirmOfAGoneDomainSaysSo() {
	remove := test.started("gone.example")
	remove.ConfirmWith("gone.example")
	Assert.Equals("This domain was already removed.", remove.Problem)
}

func (test *DomainRemoveServiceTest) TestConfirmWithoutTheServerSaysSo() {
	remove := test.started("new.example.net")
	remove.ConfirmWith("new.example.net")
	Assert.Equals("The domain was not removed. Mesail could not reach the server. Try again.", remove.Problem)
	Assert.Equals(false, remove.Removing)
	Assert.Equals("new.example.net", remove.Typed)
}
