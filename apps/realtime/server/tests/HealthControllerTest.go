// @Test(HealthTestSetup, {"partition":"tests"})
type HealthControllerTest struct {
	behaviors *BehaviorContext
}

func (test *HealthControllerTest) TestGetHealthNamesTheService() {
	Assert.Equals("realtime", test.controller().GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	Assert.Equals("ok", test.controller().GetHealth().Status)
}

func (test *HealthControllerTest) controller() *HealthController {
	return &HealthController{health: &HealthService{}}
}
