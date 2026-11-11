// @Test(DomainRemovingTestSetup, {"partition":"tests"})
type DomainRemovingTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainRemovingTest) list() *DomainListService {
	session := &AdminSessionService{}
	test.construct(session)
	list := &DomainListService{session: session, problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	return list
}

func (test *DomainRemovingTest) TestTheListShowsTheDomainAsRemoving() {
	list := test.list()
	list.Opened(AdminViewOpened{View: "domains"})
	Assert.Equals("ready", list.Step)
	Assert.Equals("2 domains", list.Heading)
	Assert.Equals(true, list.Removing)
	Assert.Equals("Removing", list.Rows[1].StateText)
	Assert.Equals(true, list.Rows[1].ActionOff)
	Assert.Equals(false, list.Rows[0].ActionOff)
}

func (test *DomainRemovingTest) TestRefreshKeepsTheRemovingState() {
	list := test.list()
	list.Refresh()
	Assert.Equals(true, list.Removing)
	Assert.Equals("gone.example.org", list.Rows[1].Name)
}
