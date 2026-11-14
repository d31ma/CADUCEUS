import "strings"

// @Test(ContentFreeTestSetup, {"partition":"tests"})
type ChatPushesServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ChatPushesServiceTest) TestAStoredMessagePushesNewMessageWithNoText() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	receipt, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "person-1", "message"))
	received := setup.Received(inbox)
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals("applied", receipt.Status)
	Assert.Equals([]map[string]any{setup.Expected("New message", "You have a new message in "+domainId+".", domainId+"/chat", "chat.message")}, received)
	Assert.IsTrue(!strings.Contains(setup.Text(received), "Secret-Name"), "the name was pushed")
	Assert.IsTrue(!strings.Contains(setup.Text(received), "env_"), "the envelope id was pushed")
}

func (test *ChatPushesServiceTest) TestEveryDeviceSubscriptionOfThePersonGetsThePush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	setup.Subscribe(keys, domainId, "person-1")
	_, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "person-1", "message"))
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals(2, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestAPersonOutsideTheDomainGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Subscribe(keys, domainId, "stranger")
	receipt, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "stranger", "message"))
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals("applied", receipt.Status)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestADisabledMemberGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "disabled")
	setup.Subscribe(keys, domainId, "person-1")
	_, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "person-1", "message"))
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestAGroupChangeGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	_, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "person-1", "group"))
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestALinkingEnvelopeGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	_, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "person-1", "linking"))
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestAnActiveMemberGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, installWide := setup.Started()
	setup.IdentityServer(member, installWide)
	server, keys, inbox := setup.PushServer(201)
	presence := setup.PresenceServer(map[string]string{"person-1": "active"})
	setup.WithPresence(presence.URL)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	_, err := setup.Events(setup.ClientFor(server)).Receive(setup.Service(member), setup.Stored(domainId, "person-1", "message"))
	Assert.IsTrue(err == nil, "event failed")
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestTheSameEventPushesOnce() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	events := setup.Events(setup.ClientFor(server))
	envelope := setup.Stored(domainId, "person-1", "message")
	_, first := events.Receive(setup.Service(member), envelope)
	receipt, second := events.Receive(setup.Service(member), envelope)
	Assert.IsTrue(first == nil, "first failed")
	Assert.IsTrue(second == nil, "second failed")
	Assert.Equals("duplicate", receipt.Status)
	Assert.Equals(1, len(setup.Received(inbox)))
}

func (test *ChatPushesServiceTest) TestAnUndeliveredNoticeGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, _, _ := setup.Started()
	err := setup.ChatPushes(nil).EnvelopeStored(setup.Kit(setup.Stored(domainId, "person-1", "undelivered")))
	Assert.IsTrue(err == nil, "undelivered failed")
}
