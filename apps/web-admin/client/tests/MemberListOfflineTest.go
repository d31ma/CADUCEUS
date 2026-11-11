// @Test(MemberOfflineDirectoryTestSetup, {"partition":"tests"})
type MemberListOfflineTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *MemberListOfflineTest) TestOpenedWithoutTheServerShowsTheError() {
	list := &MemberListService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	Assert.Equals("failed", list.State)
}

func (test *MemberListOfflineTest) TestTheAliasFormSaysTheServerIsUnavailable() {
	form := &AliasFormService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(form)
	form.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	Assert.Equals("unavailable", form.Problem)
	Assert.Equals(0, len(form.Choices))
}

func (test *MemberListOfflineTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}
