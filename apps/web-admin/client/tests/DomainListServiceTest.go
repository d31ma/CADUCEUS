// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainListServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainListServiceTest) list() *DomainListService {
	session := &AdminSessionService{}
	test.construct(session)
	list := &DomainListService{session: session, problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	return list
}

func (test *DomainListServiceTest) TestOpenedListsEveryDomain() {
	list := test.list()
	list.Opened(AdminViewOpened{View: "domains", DomainId: "mesail-operators"})
	Assert.Equals("ready", list.Step)
	Assert.Equals(3, len(list.Rows))
	Assert.Equals("example.com", list.Rows[0].Name)
	Assert.Equals("new.example.net", list.Rows[1].DomainId)
}

func (test *DomainListServiceTest) TestOpenedCountsTheDomains() {
	list := test.list()
	list.Opened(AdminViewOpened{View: "domains"})
	Assert.Equals("3 domains", list.Heading)
	Assert.Equals("", list.ProblemText)
}

func (test *DomainListServiceTest) TestRowsShowStateAndMarks() {
	list := test.list()
	list.Opened(AdminViewOpened{View: "domains"})
	Assert.Equals("Pending", list.Rows[1].StateText)
	Assert.Equals("Set up DNS", list.Rows[1].ActionText)
	Assert.Equals("ms-mark ms-mark--missing", list.Rows[1].Marks[2].MarkClass)
	Assert.Equals("DNS: MX not checked, SPF not checked, DKIM not checked, DMARC not checked", list.Rows[2].MarksLabel)
}

func (test *DomainListServiceTest) TestRefreshLoadsAgain() {
	list := test.list()
	list.Refresh()
	Assert.Equals("ready", list.Step)
	Assert.Equals(3, len(list.Rows))
}
