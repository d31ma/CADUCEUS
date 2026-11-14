// @Test(HealthTestSetup, {"partition":"tests"})
type HealthControllerTest struct {
	behaviors *BehaviorContext
}

func (test *HealthControllerTest) TestGetHealthNamesTheService() {
	Assert.Equals("notify", HealthTestSetup{}.Health().GetHealth().Service)
}

func (test *HealthControllerTest) TestGetHealthSaysOk() {
	Assert.Equals("ok", HealthTestSetup{}.Health().GetHealth().Status)
}
