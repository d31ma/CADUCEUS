import "errors"

// @Test(HealthTestSetup, {"partition":"tests"})
type JobsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *JobsControllerTest) TestPostSweepRefusesAMissingToken() {
	controller := &JobsController{sweeps: &SweepService{failures: &ErrorMappingService{}}}
	_, failure := controller.PostSweep("")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "a sweep ran without a token")
	Assert.Equals(5011, refused.Number())
}
