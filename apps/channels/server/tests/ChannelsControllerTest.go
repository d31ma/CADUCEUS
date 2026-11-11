// @Test(ChannelsTestSetup, {"partition":"tests"})
type ChannelsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelsControllerTest) TestEveryRouteRefusesAForgedToken() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	controller := world.Controller
	_, listError := controller.GetChannels("Bearer forged")
	_, createError := controller.PostChannel("Bearer forged", CreateChannelRequest{Name: "releases", Visibility: "public"})
	joinError := controller.PostJoin("Bearer forged", "ch_1")
	leaveError := controller.PostLeave("Bearer forged", "ch_1")
	inviteError := controller.PostInvite("Bearer forged", "ch_1", AddMembersRequest{MemberIds: []string{"bo"}})
	_, patchError := controller.PatchChannel("Bearer forged", "ch_1", UpdateChannelRequest{})
	Assert.IsTrue(world.IsUnauthenticated(listError), "list: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(createError), "create: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(joinError), "join: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(leaveError), "leave: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(inviteError), "invite: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(patchError), "patch: want Unauthenticated")
}
