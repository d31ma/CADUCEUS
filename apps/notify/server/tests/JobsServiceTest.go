import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Test(JobsTestSetup, {"partition":"tests"})
type JobsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *JobsServiceTest) TestRunRefusesADomainService() {
	setup := JobsTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := JobsTestSetup{}.Jobs(nil, outbox.NewRecordingPusher()).Run(setup.Service(member))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *JobsServiceTest) TestRunRefusesABadToken() {
	setup := JobsTestSetup{}
	setup.Realms(setup.Domain())
	_, err := JobsTestSetup{}.Jobs(nil, outbox.NewRecordingPusher()).Run("Bearer nonsense")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused))
}

func (test *JobsServiceTest) TestRunVisitsTheRegisteredDomains() {
	JobsTestSetup{}.FreshRegistries()
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	_, operators := setup.Realms(domainId)
	server, _ := JobsTestSetup{}.PushServer(201)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, "")
	setup.Domains().Register(domainId)
	report, err := JobsTestSetup{}.Jobs(JobsTestSetup{}.ClientFor(server), outbox.NewRecordingPusher()).Run(setup.Service(operators))
	Assert.IsTrue(err == nil, "run failed")
	Assert.IsTrue(report.Domains >= 1 || report.More, "no domain was visited")
}
