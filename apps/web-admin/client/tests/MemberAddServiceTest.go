// @Test(MemberDirectoryTestSetup, {"partition":"tests"})
type MemberAddServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *MemberAddServiceTest) opened() *MemberAddService {
	session := &AdminSessionService{DomainId: "example.com"}
	test.construct(session)
	list := &MemberListService{session: session, problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	add := &MemberAddService{session: session, memberList: list, problems: &AdminProblemService{}, rules: &UsernameRuleService{}}
	test.construct(add)
	session.DomainId = "example.com"
	add.Open()
	return add
}

func (test *MemberAddServiceTest) TestOpenAsksForTheUsername() {
	add := test.opened()
	Assert.Equals("username", add.Step)
	Assert.Equals("example.com", add.DomainId)
	Assert.Equals(false, add.FirstAdmin)
}

func (test *MemberAddServiceTest) TestOpenWithFillsTheUsername() {
	add := test.opened()
	add.OpenWith("zoe")
	Assert.Equals("zoe", add.Username)
	Assert.Equals("username", add.Step)
}

func (test *MemberAddServiceTest) TestAnExistingPersonIsAddedAsTheyAre() {
	add := test.opened()
	add.AddWith(" Eve ", false)
	Assert.Equals("added", add.Step)
	Assert.Equals(true, add.ExistingPerson)
	Assert.Equals("Eve Park", add.AddedName)
	Assert.Equals("Eve", add.AddedFirstName)
	Assert.Equals("eve@example.com", add.AddedAddress)
}

func (test *MemberAddServiceTest) TestANewUsernameAsksForANameAndAPassword() {
	add := test.opened()
	add.AddWith("dan", true)
	Assert.Equals("details", add.Step)
	Assert.Equals(true, add.Admin)
	Assert.Equals("", add.UsernameProblem)
}

func (test *MemberAddServiceTest) TestCreateWithMakesTheNewAccount() {
	add := test.opened()
	add.AddWith("dan", false)
	add.CreateWith(" Dan Example ", "a long first password")
	Assert.Equals("added", add.Step)
	Assert.Equals(false, add.ExistingPerson)
	Assert.Equals("Dan Example", add.AddedName)
	Assert.Equals("dan@example.com", add.AddedAddress)
}

func (test *MemberAddServiceTest) TestCreateWithChecksTheNameAndThePassword() {
	add := test.opened()
	add.AddWith("dan", false)
	add.CreateWith(" ", "short")
	Assert.Equals("details", add.Step)
	Assert.Equals("Type the name people see, like Dan Example.", add.NameProblem)
	Assert.Equals("Use at least 12 characters.", add.PasswordProblem)
}

func (test *MemberAddServiceTest) TestAPasswordThePolicyRefusesStaysOnStepTwo() {
	add := test.opened()
	add.AddWith("dan", false)
	add.CreateWith("Dan Example", "weakpassword1")
	Assert.Equals("details", add.Step)
	Assert.Equals("Mesail did not accept this password. Use at least 12 characters, and not the username or the address.", add.PasswordProblem)
}

func (test *MemberAddServiceTest) TestBackGoesToTheUsername() {
	add := test.opened()
	add.AddWith("dan", false)
	add.CreateWith(" ", "short")
	add.Back()
	Assert.Equals("username", add.Step)
	Assert.Equals("", add.PasswordProblem)
}

func (test *MemberAddServiceTest) TestTheBrowserChecksThePrefixRules() {
	add := test.opened()
	add.AddWith("dan smith", false)
	Assert.Equals("username", add.Step)
	Assert.Equals("Use 1 to 64 letters, digits, dots, dashes or underscores. Start and end with a letter or digit.", add.UsernameProblem)
}

func (test *MemberAddServiceTest) TestAMemberAlreadyListedIsCheckedBeforeSending() {
	add := test.opened()
	add.AddWith("bo", false)
	Assert.Equals("username", add.Step)
	Assert.Equals("Bo Lee is already a member of example.com.", add.UsernameProblem)
}

func (test *MemberAddServiceTest) TestADuplicateAddressIsExplained() {
	add := test.opened()
	add.AddWith("sales", false)
	Assert.Equals("username", add.Step)
	Assert.Equals("sales cannot be used. It is already a member here, or an alias or channel address uses it in a domain on this install.", add.UsernameProblem)
}

func (test *MemberAddServiceTest) TestForbiddenAndOfflineShowAProblem() {
	add := test.opened()
	add.AddWith("locked", false)
	Assert.Equals("forbidden", add.Problem)
	add.AddWith("offline", false)
	Assert.Equals("unavailable", add.Problem)
	Assert.Equals("username", add.Step)
}

func (test *MemberAddServiceTest) TestAddAnotherStartsAgain() {
	add := test.opened()
	add.AddWith("eve", false)
	add.AddAnother()
	Assert.Equals("username", add.Step)
	Assert.Equals("", add.Username)
}

func (test *MemberAddServiceTest) TestCloseHidesTheDialog() {
	add := test.opened()
	add.Close()
	Assert.Equals("", add.Step)
}

func (test *MemberAddServiceTest) TestNameFirstAdminAddsAnAdminToTheNamedDomain() {
	add := test.opened()
	add.NameFirstAdmin("example.org", "eve")
	Assert.Equals("added", add.Step)
	Assert.Equals(true, add.FirstAdmin)
	Assert.Equals(true, add.Admin)
	Assert.Equals("example.org", add.DomainId)
}

func (test *MemberAddServiceTest) TestNameFirstAdminWithANewUsernameAsksForDetails() {
	add := test.opened()
	add.NameFirstAdmin("example.org", "nia")
	Assert.Equals("details", add.Step)
	add.CreateWith("Nia Obi", "a long first password")
	Assert.Equals("added", add.Step)
	Assert.Equals("Nia Obi", add.AddedName)
}
