// @Test(DomainEmptyTestSetup, {"partition":"tests"})
type DomainEmptyTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *DomainEmptyTest) TestTheListSaysThereAreNoDomains() {
	session := &AdminSessionService{}
	test.construct(session)
	list := &DomainListService{session: session, problems: &AdminProblemService{}, domainRows: &DomainRowService{records: &DomainRecordService{}}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "domains"})
	Assert.Equals("empty", list.Step)
	Assert.Equals("0 domains", list.Heading)
}
