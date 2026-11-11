// @Test(HealthTestSetup, {"partition":"tests"})
type HealthControllerTest struct {
	behaviors *BehaviorContext
}

func (test *HealthControllerTest) TestGetHealthNamesTheService() {
	Assert.Equals("mail-out", (&HealthController{}).GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	Assert.Equals("ok", (&HealthController{}).GetHealth().Status)
}
