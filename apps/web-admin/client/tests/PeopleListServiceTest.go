// @Test(PeopleDirectoryTestSetup, {"partition":"tests"})
type PeopleListServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *PeopleListServiceTest) opened() *PeopleListService {
	session := &AdminSessionService{}
	test.construct(session)
	people := &PeopleListService{session: session, problems: &AdminProblemService{}}
	test.construct(people)
	people.Opened(AdminViewOpened{View: "people", DomainId: "*"})
	return people
}

func (test *PeopleListServiceTest) TestOpenedListsEveryPerson() {
	people := test.opened()
	Assert.Equals("ready", people.State)
	Assert.Equals("3 people", people.Heading)
	Assert.Equals("Ana Ruiz", people.Rows[0].Name)
	Assert.Equals("ana", people.Rows[0].Username)
	Assert.Equals(2, len(people.Rows[0].DomainIds))
	Assert.Equals("AR", people.Rows[0].Initials)
}

func (test *PeopleListServiceTest) TestStatesHavePlainNames() {
	people := test.opened()
	Assert.Equals("Active", people.Rows[0].StateText)
	Assert.Equals("Turned off", people.Rows[1].StateText)
	Assert.Equals(true, people.Rows[1].Disabled)
	Assert.Equals("Deleted", people.Rows[2].StateText)
	Assert.Equals(true, people.Rows[2].Deleted)
	Assert.Equals(false, people.Rows[2].AccountManaged)
	Assert.Equals("success", people.Rows[0].StateColor)
	Assert.Equals("warning", people.Rows[1].StateColor)
	Assert.Equals(true, people.Rows[2].NoDomains)
	Assert.Equals(false, people.Rows[2].Deleting)
	Assert.Equals(false, people.Deleting)
}

func (test *PeopleListServiceTest) TestADeletedPersonWithDomainsLeftIsDeleting() {
	people := test.opened()
	people.SearchWith("leaving", "")
	Assert.Equals(true, people.Deleting)
	Assert.Equals("Deleting", people.Rows[0].StateText)
	Assert.Equals("error", people.Rows[0].StateColor)
	Assert.Equals(true, people.Rows[0].Deleting)
	Assert.Equals(true, people.Rows[0].Deleted)
	Assert.Equals(false, people.Rows[0].AccountManaged)
	Assert.Equals(1, len(people.Rows[0].DomainIds))
	Assert.Equals("Deleted", people.Rows[1].StateText)
	Assert.Equals(false, people.Rows[1].Deleting)
}

func (test *PeopleListServiceTest) TestTheDeletedFilterKeepsDeletingPeople() {
	people := test.opened()
	people.SearchWith("leaving", "deleted")
	Assert.Equals(2, len(people.Rows))
	Assert.Equals("Deleting", people.Rows[0].StateText)
}

func (test *PeopleListServiceTest) TestTheStateFilterKeepsMatchingPeople() {
	people := test.opened()
	people.SearchWith("", "disabled")
	Assert.Equals(1, len(people.Rows))
	Assert.Equals("1 person", people.Heading)
	Assert.Equals("disabled", people.StateFilter)
}

func (test *PeopleListServiceTest) TestAllStatesClearsTheFilter() {
	people := test.opened()
	people.SearchWith("", "all")
	Assert.Equals("", people.StateFilter)
	Assert.Equals(3, len(people.Rows))
}

func (test *PeopleListServiceTest) TestNoMatchShowsNoResults() {
	people := test.opened()
	people.SearchWith(" zoe ", "")
	Assert.Equals("noResults", people.State)
	Assert.Equals("zoe", people.SearchedQuery)
}

func (test *PeopleListServiceTest) TestMorePagesSayThePeopleAreOnlyShown() {
	people := test.opened()
	people.SearchWith("dan", "")
	Assert.Equals("1 person shown", people.Heading)
	Assert.Equals(true, people.HasMore)
}

func (test *PeopleListServiceTest) TestForbiddenAndOfflineHaveTheirStates() {
	people := test.opened()
	people.SearchWith("locked", "")
	Assert.Equals("forbidden", people.State)
	people.SearchWith("broken", "")
	Assert.Equals("failed", people.State)
}

func (test *PeopleListServiceTest) TestRetryAndClearSearchLoadAgain() {
	people := test.opened()
	people.SearchWith("broken", "active")
	people.Query = "dan"
	people.Retry()
	Assert.Equals(0, len(people.Rows))
	people.ClearSearch()
	Assert.Equals(3, len(people.Rows))
	Assert.Equals("", people.StateFilter)
}

func (test *PeopleListServiceTest) TestChangedLoadsAgain() {
	people := test.opened()
	people.Query = "zoe"
	people.Changed(PeopleChanged{PersonId: "dan-id"})
	Assert.Equals("noResults", people.State)
}
