// @Test(ContentFreeTestSetup, {"partition":"tests"})
type CallPushesControllerTest struct {
	behaviors *BehaviorContext
}

func (test *CallPushesControllerTest) TestPostCallPushPushesTheMembers() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	controller := &CallPushesController{pushes: setup.CallPushes(setup.ClientFor(server))}
	report, err := controller.PostCallPush(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "post failed")
	Assert.Equals(1, report.Pushed)
	Assert.Equals(1, len(setup.Received(inbox)))
}
