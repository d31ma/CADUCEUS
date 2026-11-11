// @Test(MemberEmptyDirectoryTestSetup, {"partition":"tests"})
type MemberListEmptyTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *MemberListEmptyTest) TestADomainWithoutMembersIsEmpty() {
	list := &MemberListService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	Assert.Equals("empty", list.State)
	Assert.Equals("0 members", list.Heading)
}

func (test *MemberListEmptyTest) TestTheAliasFormHasNoMembersToChoose() {
	form := &AliasFormService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(form)
	form.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	Assert.Equals(0, len(form.Choices))
	Assert.Equals("", form.Problem)
}

func (test *MemberListEmptyTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}
