import (
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/realtime"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type ControllerFailureTest struct {
	behaviors *BehaviorContext
}

func (test *ControllerFailureTest) TestEventsMapsAnInvalidEnvelopeToBadRequest() {
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As((&EventsController{}).failure(events.InvalidEnvelope{Reason: "no id"}), &invalid), "not a bad request")
}

func (test *ControllerFailureTest) TestEventsMapsAnUnsubscribedTypeToBadRequest() {
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As((&EventsController{}).failure(events.NotSubscribed{Service: "realtime", Type: "member.upserted"}), &invalid), "not a bad request")
}

func (test *ControllerFailureTest) TestEventsMapsAMemberCallerToForbidden() {
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As((&EventsController{}).failure(events.NotService{}), &forbidden), "not forbidden")
}

func (test *ControllerFailureTest) TestEventsMapsAnotherDomainToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&EventsController{}).failure(realtime.Missing{What: "domain"}), &missing), "not missing")
}

func (test *ControllerFailureTest) TestEventsMapsAStorageFailureToUnavailable() {
	unavailable := &Unavailable{}
	Assert.IsTrue(errors.As((&EventsController{}).failure(errors.New("down")), &unavailable), "not unavailable")
}

func (test *ControllerFailureTest) TestPresenceMapsAnInvalidRequestWithItsField() {
	invalid := &InvalidRequest{}
	errors.As((&PresenceController{}).failure(realtime.Invalid{Field: "state", Reason: "bad"}), &invalid)
	Assert.Equals("state", invalid.Field)
}

func (test *ControllerFailureTest) TestPresenceMapsMissingToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&PresenceController{}).failure(realtime.Missing{What: "presence"}), &missing), "not missing")
}

func (test *ControllerFailureTest) TestJobsMapsAnotherRealmToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&JobsController{}).failure(realtime.Missing{What: "domain"}), &missing), "not missing")
}

func (test *ControllerFailureTest) TestJobsMapsAMissingRoleToForbidden() {
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As((&JobsController{}).failure(realtime.Forbidden{}), &forbidden), "not forbidden")
}
