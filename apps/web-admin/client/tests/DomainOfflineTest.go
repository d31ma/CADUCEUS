// @Test(DomainOfflineTestSetup, {"partition":"tests"})
type DomainOfflineTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainOfflineTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}

func (test *DomainOfflineTest) TestTheListSaysTheServerIsUnreachable() {
	list := &DomainListService{session: test.session(), problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "domains"})
	Assert.Equals("failed", list.Step)
	Assert.Equals("Mesail could not reach the server. Check your connection, then try again.", list.ProblemText)
	Assert.Equals(0, len(list.Rows))
}

func (test *DomainOfflineTest) TestTheDomainViewSaysTheServerIsUnreachable() {
	records := &DomainRecordService{}
	dns := &DomainDnsService{session: test.session(), problems: &AdminProblemService{}, domainRows: &DomainRowService{records: records}}
	test.construct(dns)
	dns.Opened(AdminViewOpened{View: "domain", DomainId: "example.com"})
	Assert.Equals("failed", dns.Step)
	Assert.Equals("Mesail could not reach the server. Check your connection, then try again.", dns.ProblemText)
}
