// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainDnsServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainDnsServiceTest) opened(domainId string) *DomainDnsService {
	session := &AdminSessionService{}
	test.construct(session)
	records := &DomainRecordService{}
	dns := &DomainDnsService{session: session, problems: &AdminProblemService{}, domainRows: &DomainRowService{records: records}}
	test.construct(dns)
	dns.Opened(AdminViewOpened{View: "domain", DomainId: domainId})
	return dns
}

func (test *DomainDnsServiceTest) TestOpenedShowsThePendingDomain() {
	dns := test.opened("new.example.net")
	Assert.Equals("ready", dns.Step)
	Assert.Equals("new.example.net", dns.Name)
	Assert.Equals("Pending", dns.StateText)
	Assert.Equals("ms-tag ms-tag--warn", dns.StateClass)
	Assert.Equals(true, dns.Pending)
}

func (test *DomainDnsServiceTest) TestOpenedShowsTheFourRecords() {
	dns := test.opened("new.example.net")
	Assert.Equals(4, len(dns.Records))
	Assert.Equals("ms2026a._domainkey.new.example.net", dns.Records[2].Host)
	Assert.Equals("2 of 4 records are OK.", dns.Status)
	Assert.Equals("MX is OK. DKIM is missing. Create or fix the records below, then check again. People can join this domain after mail is accepted.", dns.Waiting)
}

func (test *DomainDnsServiceTest) TestAnActiveDomainIsNotPending() {
	dns := test.opened("example.com")
	Assert.Equals("Active", dns.StateText)
	Assert.Equals(false, dns.Pending)
	Assert.Equals("4 of 4 records are OK.", dns.Status)
}

func (test *DomainDnsServiceTest) TestAnUnknownDomainIsNotFound() {
	dns := test.opened("gone.example")
	Assert.Equals("notFound", dns.Step)
	Assert.Equals("gone.example", dns.Name)
}

func (test *DomainDnsServiceTest) TestRefreshLoadsTheSameDomainAgain() {
	dns := test.opened("new.example.net")
	dns.Refresh()
	Assert.Equals("ready", dns.Step)
	Assert.Equals("new.example.net", dns.DomainId)
}

func (test *DomainDnsServiceTest) TestCheckShowsTheNewStates() {
	dns := test.opened("new.example.net")
	dns.Check()
	Assert.Equals(false, dns.Checking)
	Assert.Equals("Active", dns.StateText)
	Assert.Equals("4 of 4 records are OK.", dns.Status)
	Assert.Equals("", dns.CheckProblem)
}

func (test *DomainDnsServiceTest) TestCheckWithoutTheServerKeepsTheOldStates() {
	dns := test.opened("old.example.com")
	dns.Check()
	Assert.Equals("Disabled", dns.StateText)
	Assert.Equals("0 of 4 records are OK.", dns.Status)
	Assert.Equals("The records were not checked. Mesail could not reach the server. Check your connection, then try again.", dns.CheckProblem)
}

func (test *DomainDnsServiceTest) TestCheckWithoutOperatorRightsSaysSo() {
	dns := test.opened("example.com")
	dns.Check()
	Assert.Equals("Only operators can check DNS records.", dns.CheckProblem)
	Assert.Equals("Active", dns.StateText)
}
