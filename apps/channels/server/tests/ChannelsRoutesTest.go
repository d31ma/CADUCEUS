import "slices"

// @Test(ChannelsTestSetup, {"partition":"tests"})
type ChannelsRoutesTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelsRoutesTest) TestAMemberCreatesAChannelThatAnotherMemberListsAndJoins() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	listed, listError := world.Controller.GetChannels(world.Member("bo"))
	joinError := world.Controller.PostJoin(world.Member("bo"), created.ChannelId)
	Assert.IsTrue(listError == nil, "list failed")
	Assert.Equals([]Channel{created}, listed)
	Assert.IsTrue(joinError == nil, "join failed")
	Assert.Equals("cnv_"+created.ChannelId[len("ch_"):], created.ConversationId)
}

func (test *ChannelsRoutesTest) TestCreatingPublishesTheChannelAndTheCreatorJoining() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	Assert.Equals(1, len(world.Published("messaging", "channel.created", created.ChannelId)))
	Assert.Equals(1, len(world.Published("search", "channel.created", created.ChannelId)))
	Assert.Equals(1, len(world.Published("realtime", "channel.member.added", `"memberId":"ana"`)))
}

func (test *ChannelsRoutesTest) TestAPrivateChannelIsHiddenFromOtherMembers() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "secret", "private")
	Assert.IsTrue(!slices.Contains(world.ListedNames(world.Member("bo")), "secret"), "bo sees the private channel")
	Assert.IsTrue(world.IsNotFound(world.Controller.PostJoin(world.Member("bo"), created.ChannelId)), "want NotFound")
}

func (test *ChannelsRoutesTest) TestAMemberOfAnotherDomainNeitherSeesNorJoinsTheChannel() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	Assert.Equals(0, len(world.ListedNames(world.Stranger("eve"))))
	Assert.IsTrue(world.IsNotFound(world.Controller.PostJoin(world.Stranger("eve"), created.ChannelId)), "want NotFound")
}

func (test *ChannelsRoutesTest) TestInvitingMembersOfTheSameDomainAddsThem() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	world.Directory.Knows("bo")
	created := world.Create(world.Member("ana"), "secret", "private")
	err := world.Controller.PostInvite(world.Member("ana"), created.ChannelId, AddMembersRequest{MemberIds: []string{"bo"}})
	Assert.IsTrue(err == nil, "invite failed")
	Assert.IsTrue(slices.Contains(world.ListedNames(world.Member("bo")), "secret"), "bo does not see the channel he was invited to")
}

func (test *ChannelsRoutesTest) TestInvitingAMemberOfAnotherDomainIsRefused() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	world.Directory.Knows("bo")
	created := world.Create(world.Member("ana"), "secret", "private")
	err := world.Controller.PostInvite(world.Member("ana"), created.ChannelId, AddMembersRequest{MemberIds: []string{"bo", "eve"}})
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
	Assert.IsTrue(!slices.Contains(world.ListedNames(world.Member("bo")), "secret"), "bo was added although the invite was refused")
}

func (test *ChannelsRoutesTest) TestAMemberLeavesAndTheCreatorArchivesThroughTheRoutes() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	world.Must(world.Controller.PostJoin(world.Member("bo"), created.ChannelId))
	leaveError := world.Controller.PostLeave(world.Member("bo"), created.ChannelId)
	archived := true
	updated, err := world.Controller.PatchChannel(world.Member("ana"), created.ChannelId, UpdateChannelRequest{Archived: &archived})
	Assert.IsTrue(leaveError == nil, "leave failed")
	Assert.IsTrue(err == nil, "archive failed")
	Assert.IsTrue(updated.Archived, "not archived")
	Assert.Equals(1, len(world.Published("search", "channel.archived", created.ChannelId)))
	Assert.IsTrue(world.IsConflict(world.Controller.PostJoin(world.Member("bo"), created.ChannelId)), "want Conflict")
}

func (test *ChannelsRoutesTest) TestAnotherMemberCannotArchive() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	archived := true
	_, err := world.Controller.PatchChannel(world.Member("bo"), created.ChannelId, UpdateChannelRequest{Archived: &archived})
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *ChannelsRoutesTest) TestADomainAdminGivesTheChannelAnEmailAddress() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	address := "Releases@" + world.Domain
	updated, err := world.Controller.PatchChannel(world.Admin("root"), created.ChannelId, UpdateChannelRequest{EmailAddress: &address})
	Assert.IsTrue(err == nil, "setting the address failed")
	Assert.Equals("releases@"+world.Domain, updated.EmailAddress)
	Assert.Equals(created.ChannelId, world.Directory.ClaimedBy("releases@"+world.Domain))
}

func (test *ChannelsRoutesTest) TestATakenEmailAddressIsADuplicate() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	created := world.Create(world.Member("ana"), "releases", "public")
	address := "bo@" + world.Domain
	world.Directory.Take(address)
	_, err := world.Controller.PatchChannel(world.Admin("root"), created.ChannelId, UpdateChannelRequest{EmailAddress: &address})
	Assert.IsTrue(world.IsDuplicate(err), "want Duplicate")
}

func (test *ChannelsRoutesTest) TestAnInvalidNameIsRefusedWithItsField() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	_, err := world.Controller.PostChannel(world.Member("ana"), CreateChannelRequest{Name: "Bad Name", Visibility: "public"})
	Assert.Equals("name", world.InvalidField(err))
}

func (test *ChannelsRoutesTest) TestATokenWithoutAMemberRoleIsForbidden() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	_, err := world.Controller.GetChannels(world.Bearer(world.Home, "robot", "mesail-service"))
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}
