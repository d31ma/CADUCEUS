// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainAddServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainAddServiceTest) dialog() *DomainAddService {
	session := &AdminSessionService{}
	test.construct(session)
	shell := &AdminShellService{}
	test.construct(shell)
	list := &DomainListService{session: session, problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	add := &DomainAddService{session: session, problems: &AdminProblemService{}, adminShell: shell, domainList: list}
	test.construct(add)
	add.Show()
	return add
}

func (test *DomainAddServiceTest) TestShowOpensAnEmptyForm() {
	add := test.dialog()
	Assert.Equals(true, add.Open)
	Assert.Equals("form", add.Step)
	Assert.Equals("", add.Name)
	Assert.Equals("", add.Problem)
}

func (test *DomainAddServiceTest) TestShowForgetsTheLastProblem() {
	add := test.dialog()
	add.AddWith("new example.net")
	add.Show()
	Assert.Equals("", add.Problem)
	Assert.Equals(false, add.HasProblem)
	Assert.Equals("", add.Name)
}

func (test *DomainAddServiceTest) TestCloseHidesTheDialog() {
	add := test.dialog()
	add.Close()
	Assert.Equals(false, add.Open)
}

func (test *DomainAddServiceTest) TestAddAddsTheDomainAsPending() {
	add := test.dialog()
	add.AddWith("fresh.example.net")
	Assert.Equals("added", add.Step)
	Assert.Equals("fresh.example.net", add.AddedId)
	Assert.Equals("fresh.example.net", add.AddedName)
	Assert.Equals("", add.Problem)
	Assert.Equals("Adding fresh.example.net and making its DKIM key…", add.BusyText)
}

func (test *DomainAddServiceTest) TestAddReloadsTheList() {
	add := test.dialog()
	add.AddWith("fresh.example.net")
	Assert.Equals("ready", add.domainList.Step)
	Assert.Equals(3, len(add.domainList.Rows))
}

func (test *DomainAddServiceTest) TestAddTrimsAndLowersTheName() {
	add := test.dialog()
	add.AddWith("  Fresh.Example.NET ")
	Assert.Equals("fresh.example.net", add.Name)
	Assert.Equals("added", add.Step)
}

func (test *DomainAddServiceTest) TestAddAcceptsLettersFromAnyLanguage() {
	add := test.dialog()
	add.AddWith("bücher.example")
	Assert.Equals("added", add.Step)
}

func (test *DomainAddServiceTest) TestAddAsksForAName() {
	add := test.dialog()
	add.AddWith("   ")
	Assert.Equals("form", add.Step)
	Assert.Equals("Enter a domain name, like example.net.", add.Problem)
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddRefusesASpace() {
	add := test.dialog()
	add.AddWith("new example.net")
	Assert.Equals("form", add.Step)
	Assert.Equals("This is not a valid domain name. Remove the space. Use letters, digits, hyphens and dots, like example.net.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesAnAddress() {
	add := test.dialog()
	add.AddWith("ana@example.net")
	Assert.Equals("Use the domain only, without www or @.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesWww() {
	add := test.dialog()
	add.AddWith("www.example.net")
	Assert.Equals("Use the domain only, without www or @.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesANameWithoutADot() {
	add := test.dialog()
	add.AddWith("localhost")
	Assert.Equals("This is not a valid domain name. Use letters, digits, hyphens and dots, like example.net.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesALabelThatStartsWithAHyphen() {
	add := test.dialog()
	add.AddWith("-bad.example")
	Assert.Equals("form", add.Step)
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddRefusesTwoDotsInARow() {
	add := test.dialog()
	add.AddWith("bad..example")
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddRefusesOtherSigns() {
	add := test.dialog()
	add.AddWith("bad_name.example")
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddSaysWhenTheDomainIsAlreadyAdded() {
	add := test.dialog()
	add.AddWith("taken.example")
	Assert.Equals("form", add.Step)
	Assert.Equals("This domain is already added to Mesail.", add.Problem)
	Assert.Equals("", add.AddedId)
}

func (test *DomainAddServiceTest) TestAddSaysWhenTheServerRefusesTheName() {
	add := test.dialog()
	add.AddWith("rude.example")
	Assert.Equals("This is not a valid domain name. Use letters, digits, hyphens and dots, like example.net.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddSaysWhenOperatorRightsAreMissing() {
	add := test.dialog()
	add.AddWith("blocked.example")
	Assert.Equals("You need operator rights to add domains.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddSaysWhenTheServerIsUnreachable() {
	add := test.dialog()
	add.AddWith("offline.example")
	Assert.Equals("form", add.Step)
	Assert.Equals("The domain was not added. Mesail could not reach the server. Try again.", add.Problem)
}

func (test *DomainAddServiceTest) TestShowDnsOpensTheDomain() {
	add := test.dialog()
	add.AddWith("fresh.example.net")
	add.ShowDns()
	Assert.Equals(false, add.Open)
	Assert.Equals("domain", add.adminShell.View)
	Assert.Equals("fresh.example.net", add.adminShell.DomainId)
}
