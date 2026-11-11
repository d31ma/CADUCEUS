// @Test(HealthTestSetup, {"partition":"tests"})
type PresenceControllerTest struct {
	behaviors *BehaviorContext
}

func (test *PresenceControllerTest) TestPutPresenceRefusesAMissingToken() {
	failure := (&PresenceController{}).PutPresence("", PresenceRequest{State: "idle"})
	Assert.IsTrue(failure != nil, "presence was set without a token")
}

func (test *PresenceControllerTest) TestGetPresenceRefusesAMissingToken() {
	found, failure := (&PresenceController{}).GetPresence("", "ana")
	Assert.IsTrue(failure != nil, "presence was read without a token")
	Assert.Equals(0, len(found))
}
