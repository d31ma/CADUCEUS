// @Test(MessagingTestSetup, {"partition":"tests"})
type CallerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CallerServiceTest) TestChatCallerAcceptsAMember() {
	world := MessagingTestSetup{}.World()
	caller, err := (&CallerService{}).ChatCaller(world.MemberBearer("bo"))
	Assert.IsTrue(err == nil, "a member was refused")
	Assert.Equals(world.Domain, caller.DomainId)
	Assert.Equals("bo", caller.MemberId)
}

func (test *CallerServiceTest) TestChatCallerAcceptsADomainAdmin() {
	world := MessagingTestSetup{}.World()
	_, err := (&CallerService{}).ChatCaller(world.Bearer(world.Home, "ana", "mesail-admin"))
	Assert.IsTrue(err == nil, "a domain admin was refused")
}

func (test *CallerServiceTest) TestChatCallerRefusesAServiceToken() {
	world := MessagingTestSetup{}.World()
	_, err := (&CallerService{}).ChatCaller(world.ServiceBearer(world.Home))
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *CallerServiceTest) TestChatCallerRefusesAForgedToken() {
	world := MessagingTestSetup{}.World()
	_, err := (&CallerService{}).ChatCaller("Bearer forged")
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}

func (test *CallerServiceTest) TestServiceCallerAcceptsAServiceToken() {
	world := MessagingTestSetup{}.World()
	caller, err := (&CallerService{}).ServiceCaller(world.ServiceBearer(world.Home))
	Assert.IsTrue(err == nil, "a service was refused")
	Assert.Equals(world.Domain, caller.DomainId)
}

func (test *CallerServiceTest) TestServiceCallerRefusesAMember() {
	world := MessagingTestSetup{}.World()
	_, err := (&CallerService{}).ServiceCaller(world.MemberBearer("bo"))
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *CallerServiceTest) TestServiceCallerRefusesAMissingToken() {
	world := MessagingTestSetup{}.World()
	_, err := (&CallerService{}).ServiceCaller("")
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}
