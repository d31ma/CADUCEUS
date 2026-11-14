// @Test(HealthTestSetup, {"partition":"tests"})
type HealthControllerTest struct {
	behaviors *BehaviorContext
}

func (test *HealthControllerTest) TestGetHealthNamesTheService() {
	Assert.Equals("devicebox", (&HealthController{health: &HealthService{}}).GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	Assert.Equals("ok", (&HealthController{health: &HealthService{}}).GetHealth().Status)
}
