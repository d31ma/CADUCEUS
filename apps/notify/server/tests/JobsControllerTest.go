import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Test(JobsTestSetup, {"partition":"tests"})
type JobsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *JobsControllerTest) TestPostRunHidesTheRouteFromADomainService() {
	setup := JobsTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	controller := &JobsController{jobs: JobsTestSetup{}.Jobs(nil, outbox.NewRecordingPusher())}
	_, err := controller.PostRun(setup.Service(member))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}
