// @Test(MemberDirectoryTestSetup, {"partition":"tests"})
type MemberRoleServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *MemberRoleServiceTest) role() *MemberRoleService {
	session := &AdminSessionService{DomainId: "example.com"}
	test.construct(session)
	role := &MemberRoleService{session: session, problems: &AdminProblemService{}}
	test.construct(role)
	return role
}

func (test *MemberRoleServiceTest) TestSetRoleChangesTheRole() {
	role := test.role()
	role.SetRole("dan-id", "Dan Example", true)
	Assert.Equals("", role.Dialog)
	Assert.Equals("", role.Problem)
	Assert.Equals(false, role.Busy)
}

func (test *MemberRoleServiceTest) TestTheLastAdminKeepsTheRole() {
	role := test.role()
	role.SetRole("bo-id", "Bo Lee", false)
	Assert.Equals("lastAdmin", role.Dialog)
	Assert.Equals("role", role.LastAdminFor)
	Assert.Equals("example.com", role.DomainId)
}

func (test *MemberRoleServiceTest) TestAskRemoveOpensTheConfirmation() {
	role := test.role()
	role.AskRemove("dan-id", "Dan Example", "dan@example.com")
	Assert.Equals("remove", role.Dialog)
	Assert.Equals("dan@example.com", role.Address)
}

func (test *MemberRoleServiceTest) TestRemoveRemovesTheMember() {
	role := test.role()
	role.AskRemove("dan-id", "Dan Example", "dan@example.com")
	role.Remove()
	Assert.Equals("removed", role.Dialog)
}

func (test *MemberRoleServiceTest) TestAMemberAlreadyGoneCountsAsRemoved() {
	role := test.role()
	role.AskRemove("gone-id", "Gone", "gone@example.com")
	role.Remove()
	Assert.Equals("removed", role.Dialog)
}

func (test *MemberRoleServiceTest) TestTheLastAdminCannotBeRemoved() {
	role := test.role()
	role.AskRemove("bo-id", "Bo Lee", "bo@example.com")
	role.Remove()
	Assert.Equals("lastAdmin", role.Dialog)
	Assert.Equals("removal", role.LastAdminFor)
}

func (test *MemberRoleServiceTest) TestForbiddenAndOfflineOpenTheirDialogs() {
	role := test.role()
	role.SetRole("eve-id", "Eve Park", true)
	Assert.Equals("forbidden", role.Dialog)
	role.SetRole("offline-id", "Off Line", true)
	Assert.Equals("failed", role.Dialog)
}

func (test *MemberRoleServiceTest) TestCloseHidesTheDialog() {
	role := test.role()
	role.AskRemove("dan-id", "Dan Example", "dan@example.com")
	role.Close()
	Assert.Equals("", role.Dialog)
}
