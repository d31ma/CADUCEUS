// @Test(DomainForbiddenTestSetup, {"partition":"tests"})
type DomainForbiddenTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainForbiddenTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}

func (test *DomainForbiddenTest) TestTheListAsksForOperatorRights() {
	list := &DomainListService{session: test.session(), problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "domains"})
	Assert.Equals("failed", list.Step)
	Assert.Equals("You need operator rights to manage domains.", list.ProblemText)
}

func (test *DomainForbiddenTest) TestADomainAdminSeesThatOnlyOperatorsSeeDnsChecks() {
	records := &DomainRecordService{}
	dns := &DomainDnsService{session: test.session(), problems: &AdminProblemService{}, records: records, domainRows: &DomainRowService{records: records}}
	test.construct(dns)
	dns.Opened(AdminViewOpened{View: "domain", DomainId: "example.com"})
	Assert.Equals("operatorsOnly", dns.Step)
	Assert.Equals("example.com", dns.Name)
	Assert.Equals("Only operators can see DNS checks for now. Ask a Mesail operator to check the DNS records of this domain.", dns.ProblemText)
}
