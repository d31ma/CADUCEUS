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

func (test *MemberListOfflineTest) opened() *AliasFormService {
	form := &AliasFormService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(form)
	form.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	return form
}

func (test *MemberListOfflineTest) TestTheAliasFormSaysTheMemberListIsUnavailable() {
	form := test.opened()
	Assert.Equals("unavailable", form.ListProblem)
	Assert.Equals(0, len(form.Choices))
}

func (test *MemberListOfflineTest) TestTheAliasFormShowsNoAddProblemBeforeASubmit() {
	form := test.opened()
	Assert.Equals("", form.Problem)
}

func (test *MemberListOfflineTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}
