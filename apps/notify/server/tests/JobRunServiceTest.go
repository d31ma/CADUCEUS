import "mesail.dev/packages/outbox"

// @Test(JobsTestSetup, {"partition":"tests"})
type JobRunServiceTest struct {
	behaviors *BehaviorContext
}

func (test *JobRunServiceTest) TestRunDomainsPushesAndSummarizes() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	server, calls := JobsTestSetup{}.PushServer(201)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, "")
	JobsTestSetup{}.MemberSettings(domainId, "member-1", 15, "ana@"+domainId)
	JobsTestSetup{}.Subscribe(domainId, "member-1", "web")
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	pusher := outbox.NewRecordingPusher()
	report, err := JobsTestSetup{}.Runner(JobsTestSetup{}.ClientFor(server), pusher).RunDomains([]string{domainId}, JobsTestSetup{}.Budget(25))
	found := JobsTestSetup{}.Found(record)
	Assert.IsTrue(err == nil, "run failed")
	Assert.Equals(1, report.Domains)
	Assert.Equals(1, report.Pushed)
	Assert.Equals(1, report.Summarized)
	Assert.Equals(int32(1), calls.Load())
	Assert.Equals(1, len(pusher.PushedTo("mail-out")))
	Assert.Equals("sent", found.PushState)
	Assert.Equals("none", found.SummaryState)
}

func (test *JobRunServiceTest) TestRunDomainsStopsWhenTheBudgetIsSpent() {
	setup := JobsTestSetup{}
	setup.Realms(setup.Domain())
	report, err := JobsTestSetup{}.Runner(nil, outbox.NewRecordingPusher()).RunDomains([]string{setup.Domain(), setup.Domain()}, JobsTestSetup{}.Budget(0))
	Assert.IsTrue(err == nil, "run failed")
	Assert.Equals(0, report.Domains)
	Assert.IsTrue(report.More, "the left work was not reported")
}
