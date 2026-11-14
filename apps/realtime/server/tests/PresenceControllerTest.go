import "errors"

// @Test(HealthTestSetup, {"partition":"tests"})
type PresenceControllerTest struct {
	behaviors *BehaviorContext
}

func (test *PresenceControllerTest) TestPutPresenceRefusesAMissingToken() {
	failure := test.controller().PutPresence("", PresenceRequest{State: "idle"})
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "presence was set without a token")
	Assert.Equals(5011, refused.Number())
}

func (test *PresenceControllerTest) TestGetPresenceRefusesAMissingToken() {
	found, failure := test.controller().GetPresence("", "ana")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "presence was read without a token")
	Assert.Equals(5011, refused.Number())
	Assert.Equals(0, len(found))
}

func (test *PresenceControllerTest) controller() *PresenceController {
	return &PresenceController{presence: &PresenceService{failures: &ErrorMappingService{}}}
}
