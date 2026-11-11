// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainAddServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainAddServiceTest) dialog(name string) *DomainAddService {
	session := &AdminSessionService{}
	test.construct(session)
	shell := &AdminShellService{}
	test.construct(shell)
	list := &DomainListService{session: session, problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	add := &DomainAddService{session: session, problems: &AdminProblemService{}, adminShell: shell, domainList: list}
	test.construct(add)
	add.Show()
	add.Name = name
	return add
}

func (test *DomainAddServiceTest) TestShowOpensAnEmptyForm() {
	add := test.dialog("")
	Assert.Equals(true, add.Open)
	Assert.Equals("form", add.Step)
	Assert.Equals("", add.Name)
	Assert.Equals("", add.Problem)
}

func (test *DomainAddServiceTest) TestShowForgetsTheLastProblem() {
	add := test.dialog("new example.net")
	add.Add()
	add.Show()
	Assert.Equals("", add.Problem)
	Assert.Equals(false, add.HasProblem)
	Assert.Equals("", add.Name)
}

func (test *DomainAddServiceTest) TestCloseHidesTheDialog() {
	add := test.dialog("")
	add.Close()
	Assert.Equals(false, add.Open)
}

func (test *DomainAddServiceTest) TestAddAddsTheDomainAsPending() {
	add := test.dialog("fresh.example.net")
	add.Add()
	Assert.Equals("added", add.Step)
	Assert.Equals("fresh.example.net", add.AddedId)
	Assert.Equals("fresh.example.net", add.AddedName)
	Assert.Equals("", add.Problem)
	Assert.Equals("Adding fresh.example.net and making its DKIM key…", add.BusyText)
}

func (test *DomainAddServiceTest) TestAddReloadsTheList() {
	add := test.dialog("fresh.example.net")
	add.Add()
	Assert.Equals("ready", add.domainList.Step)
	Assert.Equals(3, len(add.domainList.Rows))
}

func (test *DomainAddServiceTest) TestAddTrimsAndLowersTheName() {
	add := test.dialog("  Fresh.Example.NET ")
	add.Add()
	Assert.Equals("fresh.example.net", add.Name)
	Assert.Equals("added", add.Step)
}

func (test *DomainAddServiceTest) TestAddAcceptsLettersFromAnyLanguage() {
	add := test.dialog("bücher.example")
	add.Add()
	Assert.Equals("added", add.Step)
}

func (test *DomainAddServiceTest) TestAddAsksForAName() {
	add := test.dialog("   ")
	add.Add()
	Assert.Equals("form", add.Step)
	Assert.Equals("Enter a domain name, like example.net.", add.Problem)
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddRefusesASpace() {
	add := test.dialog("new example.net")
	add.Add()
	Assert.Equals("form", add.Step)
	Assert.Equals("This is not a valid domain name. Remove the space. Use letters, digits, hyphens and dots, like example.net.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesAnAddress() {
	add := test.dialog("ana@example.net")
	add.Add()
	Assert.Equals("Use the domain only, without www or @.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesWww() {
	add := test.dialog("www.example.net")
	add.Add()
	Assert.Equals("Use the domain only, without www or @.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesANameWithoutADot() {
	add := test.dialog("localhost")
	add.Add()
	Assert.Equals("This is not a valid domain name. Use letters, digits, hyphens and dots, like example.net.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddRefusesALabelThatStartsWithAHyphen() {
	add := test.dialog("-bad.example")
	add.Add()
	Assert.Equals("form", add.Step)
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddRefusesTwoDotsInARow() {
	add := test.dialog("bad..example")
	add.Add()
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddRefusesOtherSigns() {
	add := test.dialog("bad_name.example")
	add.Add()
	Assert.Equals(true, add.HasProblem)
}

func (test *DomainAddServiceTest) TestAddSaysWhenTheDomainIsAlreadyAdded() {
	add := test.dialog("taken.example")
	add.Add()
	Assert.Equals("form", add.Step)
	Assert.Equals("This domain is already added to Mesail.", add.Problem)
	Assert.Equals("", add.AddedId)
}

func (test *DomainAddServiceTest) TestAddSaysWhenTheServerRefusesTheName() {
	add := test.dialog("rude.example")
	add.Add()
	Assert.Equals("This is not a valid domain name. Use letters, digits, hyphens and dots, like example.net.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddSaysWhenOperatorRightsAreMissing() {
	add := test.dialog("blocked.example")
	add.Add()
	Assert.Equals("You need operator rights to add domains.", add.Problem)
}

func (test *DomainAddServiceTest) TestAddSaysWhenTheServerIsUnreachable() {
	add := test.dialog("offline.example")
	add.Add()
	Assert.Equals("form", add.Step)
	Assert.Equals("The domain was not added. Mesail could not reach the server. Try again.", add.Problem)
}

func (test *DomainAddServiceTest) TestShowDnsOpensTheDomain() {
	add := test.dialog("fresh.example.net")
	add.Add()
	add.ShowDns()
	Assert.Equals(false, add.Open)
	Assert.Equals("domain", add.adminShell.View)
	Assert.Equals("fresh.example.net", add.adminShell.DomainId)
}
