import (
	"errors"
	"strings"
)

// @Test(ContentFreeTestSetup, {"partition":"tests"})
type CallPushesServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CallPushesServiceTest) TestIncomingCallReachesEveryMemberWithNoName() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Person(domainId, "person-2", "active")
	setup.Subscribe(keys, domainId, "person-1")
	setup.Subscribe(keys, domainId, "person-2")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1", "person-2"}, Kind: "incoming"})
	received := setup.Received(inbox)
	expected := setup.Expected("Incoming call", "Someone is calling you in "+domainId+".", domainId+"/call", "call.incoming")
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 2, Skipped: 0}, report)
	Assert.Equals([]map[string]any{expected, expected}, received)
}

func (test *CallPushesServiceTest) TestMissedCallSaysMissedCall() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "missed"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(1, report.Pushed)
	Assert.Equals([]map[string]any{setup.Expected("Missed call", "You missed a call in "+domainId+".", domainId+"/call", "call.missed")}, setup.Received(inbox))
}

func (test *CallPushesServiceTest) TestNoPushHoldsTheNameOrTheId() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	_, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	received := setup.Received(inbox)
	text := setup.Text(received)
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(1, len(received))
	Assert.IsTrue(!strings.Contains(text, "Secret-Name"), "the name was pushed")
	Assert.IsTrue(!strings.Contains(text, "person-1"), "the person id was pushed")
	Assert.IsTrue(!strings.Contains(text, "ana@"), "the address was pushed")
}

func (test *CallPushesServiceTest) TestAPersonOutsideTheDomainGetsNoPushAndIsSkipped() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	setup.Subscribe(keys, domainId, "stranger")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1", "stranger"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 1, Skipped: 1}, report)
	Assert.Equals(1, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestARemovedMemberGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "removed")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "missed"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 0, Skipped: 1}, report)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestADisabledMemberGetsNoPush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "disabled")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 0, Skipped: 1}, report)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestAMemberOfAnotherDomainGetsNoPush() {
	setup := ContentFreeTestSetup{}
	_, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	otherDomainId := setup.Domain()
	setup.Person(otherDomainId, "person-1", "active")
	setup.Subscribe(keys, otherDomainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 0, Skipped: 1}, report)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestAMemberTokenIsRefused() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	setup.Person(domainId, "person-1", "active")
	_, err := setup.CallPushes(nil).Request(setup.Member(member, "person-1"), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	forbidden := &NotService{}
	Assert.IsTrue(errors.As(err, &forbidden), "not refused")
	Assert.Equals(4503, forbidden.Number())
}

func (test *CallPushesServiceTest) TestAnInstallWideTokenIsRefused() {
	setup := ContentFreeTestSetup{}
	_, _, installWide := setup.Started()
	_, err := setup.CallPushes(nil).Request(setup.Service(installWide), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing), "not refused")
	Assert.Equals(4504, missing.Number())
}

func (test *CallPushesServiceTest) TestABadTokenIsRefused() {
	setup := ContentFreeTestSetup{}
	setup.Started()
	_, err := setup.CallPushes(nil).Request("Bearer nope", CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "not refused")
	Assert.Equals(4501, refused.Number())
}

func (test *CallPushesServiceTest) TestAnUnknownKindIsRefused() {
	setup := ContentFreeTestSetup{}
	_, member, _ := setup.Started()
	_, err := setup.CallPushes(nil).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "message"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "not refused")
	Assert.Equals(4523, invalid.Number())
	Assert.Equals("kind", invalid.Field())
}

func (test *CallPushesServiceTest) TestAnEmptyListIsRefused() {
	setup := ContentFreeTestSetup{}
	_, member, _ := setup.Started()
	_, err := setup.CallPushes(nil).Request(setup.Service(member), CallPush{ToPersonIds: []string{}, Kind: "incoming"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "not refused")
	Assert.Equals(4524, invalid.Number())
	Assert.Equals("toPersonIds", invalid.Field())
}

func (test *CallPushesServiceTest) TestFiftyPeopleAreRefused() {
	setup := ContentFreeTestSetup{}
	_, member, _ := setup.Started()
	people := strings.Split(strings.Repeat("person,", 50), ",")[:50]
	_, err := setup.CallPushes(nil).Request(setup.Service(member), CallPush{ToPersonIds: people, Kind: "incoming"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "not refused")
	Assert.Equals(4524, invalid.Number())
}

func (test *CallPushesServiceTest) TestALongIdIsRefused() {
	setup := ContentFreeTestSetup{}
	_, member, _ := setup.Started()
	_, err := setup.CallPushes(nil).Request(setup.Service(member), CallPush{ToPersonIds: []string{strings.Repeat("p", 65)}, Kind: "incoming"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "not refused")
	Assert.Equals(4524, invalid.Number())
}

func (test *CallPushesServiceTest) TestAnEmptyIdIsRefused() {
	setup := ContentFreeTestSetup{}
	_, member, _ := setup.Started()
	_, err := setup.CallPushes(nil).Request(setup.Service(member), CallPush{ToPersonIds: []string{""}, Kind: "incoming"})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "not refused")
	Assert.Equals(4524, invalid.Number())
}

func (test *CallPushesServiceTest) TestDoNotDisturbSendsNoIncomingCall() {
	setup := ContentFreeTestSetup{}
	domainId, member, installWide := setup.Started()
	setup.IdentityServer(member, installWide)
	server, keys, inbox := setup.PushServer(201)
	presence := setup.PresenceServer(map[string]string{"person-1": "dnd"})
	setup.WithPresence(presence.URL)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 0, Skipped: 1}, report)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestPushTurnedOffSendsNothing() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.WithPushTurnedOff()
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 0, Skipped: 1}, report)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestAPersonNamedTwiceGetsOnePush() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1", "person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(1, report.Pushed)
	Assert.Equals(1, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestAMemberWithoutASubscriptionIsSkipped() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, _, inbox := setup.PushServer(201)
	setup.Person(domainId, "person-1", "active")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "incoming"})
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(CallPushReport{Pushed: 0, Skipped: 1}, report)
	Assert.Equals(0, len(setup.Received(inbox)))
}

func (test *CallPushesServiceTest) TestAGoneSubscriptionIsDeleted() {
	setup := ContentFreeTestSetup{}
	domainId, member, _ := setup.Started()
	server, keys, _ := setup.PushServer(410)
	setup.Person(domainId, "person-1", "active")
	setup.Subscribe(keys, domainId, "person-1")
	report, err := setup.CallPushes(setup.ClientFor(server)).Request(setup.Service(member), CallPush{ToPersonIds: []string{"person-1"}, Kind: "missed"})
	left, _ := setup.PushStore().ListForMember(domainId, "person-1")
	Assert.IsTrue(err == nil, "call push failed")
	Assert.Equals(1, report.Skipped)
	Assert.Equals(0, len(left))
}
