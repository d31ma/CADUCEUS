// @Test(MemberDirectoryTestSetup, {"partition":"tests"})
type MemberListServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *MemberListServiceTest) opened() *MemberListService {
	list := &MemberListService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	return list
}

func (test *MemberListServiceTest) TestOpenedListsTheMembersOfTheDomain() {
	list := test.opened()
	Assert.Equals("ready", list.State)
	Assert.Equals("example.com", list.DomainId)
	Assert.Equals(2, len(list.Rows))
	Assert.Equals("Ana Ruiz", list.Rows[0].Name)
	Assert.Equals("ana@example.com", list.Rows[0].Address)
	Assert.Equals("ana-id", list.Rows[0].MemberId)
}

func (test *MemberListServiceTest) TestOpenedCountsTheMembers() {
	list := test.opened()
	Assert.Equals("2 members", list.Heading)
	Assert.Equals(false, list.HasMore)
}

func (test *MemberListServiceTest) TestRowsShowTheInitialsOfTheName() {
	list := test.opened()
	Assert.Equals("AR", list.Rows[0].Initials)
	Assert.Equals("BL", list.Rows[1].Initials)
}

func (test *MemberListServiceTest) TestOpenedForgetsAnOldSearch() {
	list := &MemberListService{Query: "zoe", session: test.session(), problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	Assert.Equals("", list.Query)
	Assert.Equals("ready", list.State)
}

func (test *MemberListServiceTest) TestSearchWithNoMatchShowsNoResults() {
	list := test.opened()
	list.Query = "zoe"
	list.Search()
	Assert.Equals("noResults", list.State)
	Assert.Equals("zoe", list.SearchedQuery)
	Assert.Equals(0, len(list.Rows))
}

func (test *MemberListServiceTest) TestSearchIgnoresSpacesAroundTheQuery() {
	list := test.opened()
	list.Query = "  zoe "
	list.Search()
	Assert.Equals("zoe", list.SearchedQuery)
	Assert.Equals("noResults", list.State)
}

func (test *MemberListServiceTest) TestSearchCountsOneMemberInTheSingular() {
	list := test.opened()
	list.Query = "olu"
	list.Search()
	Assert.Equals("1 member", list.Heading)
	Assert.Equals("O", list.Rows[0].Initials)
}

func (test *MemberListServiceTest) TestMorePagesSayTheMembersAreOnlyShown() {
	list := test.opened()
	list.Query = "many"
	list.Search()
	Assert.Equals("2 members shown", list.Heading)
	Assert.Equals(true, list.HasMore)
}

func (test *MemberListServiceTest) TestForbiddenShowsNoAccess() {
	list := test.opened()
	list.Query = "locked"
	list.Search()
	Assert.Equals("forbidden", list.State)
}

func (test *MemberListServiceTest) TestANetworkFailureShowsTheError() {
	list := test.opened()
	list.Query = "broken"
	list.Search()
	Assert.Equals("failed", list.State)
	Assert.Equals(0, len(list.Rows))
}

func (test *MemberListServiceTest) TestRetryLoadsAgain() {
	list := test.opened()
	list.Query = "broken"
	list.Search()
	list.Query = "olu"
	list.Retry()
	Assert.Equals("ready", list.State)
	Assert.Equals("Olu", list.Rows[0].Name)
}

func (test *MemberListServiceTest) TestClearSearchShowsEveryone() {
	list := test.opened()
	list.Query = "zoe"
	list.Search()
	list.ClearSearch()
	Assert.Equals("", list.Query)
	Assert.Equals("ready", list.State)
	Assert.Equals(2, len(list.Rows))
}

func (test *MemberListServiceTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}
