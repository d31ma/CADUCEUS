// @Test(JobsTestSetup, {"partition":"tests"})
type PushServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PushServiceTest) prepared(status int) (string, NotificationRecord, *PushService, func() int32) {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	server, calls := JobsTestSetup{}.PushServer(status)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, "")
	JobsTestSetup{}.Subscribe(domainId, "member-1", "web")
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	return domainId, record, JobsTestSetup{}.Pushes(JobsTestSetup{}.ClientFor(server)), calls.Load
}

func (test *PushServiceTest) TestDeliverSendsTheNotification() {
	_, record, pushes, calls := test.prepared(201)
	progress, err := pushes.Deliver(record.DomainId, []NotificationRecord{record}, 25)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.Equals(1, progress.Pushed)
	Assert.Equals(int32(1), calls())
	Assert.Equals("sent", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestDeliverDeletesAGoneSubscription() {
	domainId, record, pushes, _ := test.prepared(410)
	progress, err := pushes.Deliver(domainId, []NotificationRecord{record}, 25)
	left, _ := JobsTestSetup{}.PushStore().ListForMember(domainId, "member-1")
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.Equals(1, progress.Skipped)
	Assert.Equals(0, len(left))
	Assert.Equals("skipped", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestDeliverCountsARetry() {
	domainId, record, pushes, _ := test.prepared(503)
	progress, err := pushes.Deliver(domainId, []NotificationRecord{record}, 25)
	found := JobsTestSetup{}.Found(record)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.IsTrue(progress.More, "a retry was not reported as left over")
	Assert.Equals("pending", found.PushState)
	Assert.Equals(1, found.PushAttempts)
}

func (test *PushServiceTest) TestDeliverFailsAfterTheLastAttempt() {
	setup := JobsTestSetup{}
	domainId, _, pushes, _ := test.prepared(503)
	tired := setup.Notification(domainId, "member-1", "msg_2")
	tired.PushAttempts = 4
	record := JobsTestSetup{}.Stored(tired)
	_, err := pushes.Deliver(domainId, []NotificationRecord{record}, 25)
	found := JobsTestSetup{}.Found(record)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.Equals("failed", found.PushState)
	Assert.Equals(5, found.PushAttempts)
}

func (test *PushServiceTest) TestDeliverSkipsAMemberWithoutSubscriptions() {
	setup := JobsTestSetup{}
	domainId, _, pushes, calls := test.prepared(201)
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-2", "msg_3"))
	progress, err := pushes.Deliver(domainId, []NotificationRecord{record}, 25)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.Equals(1, progress.Skipped)
	Assert.Equals(int32(0), calls())
	Assert.Equals("skipped", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestDeliverSkipsAnActiveMember() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Keycloak(setup.Realms(domainId))
	server, calls := JobsTestSetup{}.PushServer(201)
	presence := JobsTestSetup{}.PresenceServer(map[string]string{"member-1": "active"}, 200)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, presence.URL)
	JobsTestSetup{}.Subscribe(domainId, "member-1", "web")
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	progress, err := JobsTestSetup{}.Pushes(JobsTestSetup{}.ClientFor(server)).Deliver(domainId, []NotificationRecord{record}, 25)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.Equals(1, progress.Skipped)
	Assert.Equals(int32(0), calls.Load())
	Assert.Equals("skipped", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestDeliverWaitsWhenPresenceIsUnavailable() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Keycloak(setup.Realms(domainId))
	server, calls := JobsTestSetup{}.PushServer(201)
	presence := JobsTestSetup{}.PresenceServer(map[string]string{}, 500)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, presence.URL)
	JobsTestSetup{}.Subscribe(domainId, "member-1", "web")
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	progress, err := JobsTestSetup{}.Pushes(JobsTestSetup{}.ClientFor(server)).Deliver(domainId, []NotificationRecord{record}, 25)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.IsTrue(progress.More, "the push was not left for later")
	Assert.Equals(int32(0), calls.Load())
	Assert.Equals("pending", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestDeliverSkipsWhenVapidIsNotConfigured() {
	domainId, record, pushes, calls := test.prepared(201)
	JobsTestSetup{}.WithoutVapid()
	progress, err := pushes.Deliver(domainId, []NotificationRecord{record}, 25)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.Equals(1, progress.Skipped)
	Assert.Equals(int32(0), calls())
	Assert.Equals("skipped", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestDeliverStopsWhenTheBudgetIsSpent() {
	domainId, record, pushes, calls := test.prepared(201)
	progress, err := pushes.Deliver(domainId, []NotificationRecord{record}, 0)
	Assert.IsTrue(err == nil, "deliver failed")
	Assert.IsTrue(progress.More, "the push was not left for later")
	Assert.Equals(int32(0), calls())
	Assert.Equals("pending", JobsTestSetup{}.Found(record).PushState)
}

func (test *PushServiceTest) TestRunPendingSendsStoredPushes() {
	domainId, record, pushes, calls := test.prepared(201)
	progress, err := pushes.RunPending(domainId, JobsTestSetup{}.Budget(25))
	Assert.IsTrue(err == nil, "run failed")
	Assert.Equals(1, progress.Pushed)
	Assert.Equals(int32(1), calls())
	Assert.Equals("sent", JobsTestSetup{}.Found(record).PushState)
}
