// @Test(HealthTestSetup, {"partition":"tests"})
type JobsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *JobsControllerTest) TestPostSweepRefusesAMissingToken() {
	_, failure := (&JobsController{}).PostSweep("")
	Assert.IsTrue(failure != nil, "a sweep ran without a token")
}
