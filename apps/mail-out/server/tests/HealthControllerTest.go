// @Test(HealthTestSetup, {"partition":"tests"})
type HealthControllerTest struct {
	behaviors *BehaviorContext
}

func (test *HealthControllerTest) controller() *HealthController {
	return &HealthController{health: &HealthService{}}
}

func (test *HealthControllerTest) TestGetHealthNamesTheService() {
	Assert.Equals("mail-out", test.controller().GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	Assert.Equals("ok", test.controller().GetHealth().Status)
}
