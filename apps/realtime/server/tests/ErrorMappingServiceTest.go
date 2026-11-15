import (
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/realtime"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type ErrorMappingServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ErrorMappingServiceTest) TestEventsMapsAnInvalidEnvelopeToBadRequest() {
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(events.InvalidEnvelope{}.EmptyEventId()), &invalid), "not a bad request")
	Assert.Equals(1, invalid.Number())
	Assert.Equals("envelope", invalid.Field)
	Assert.Equals("eventId must not be empty", invalid.Message)
}

func (test *ErrorMappingServiceTest) TestEventsMapsAnUnsubscribedTypeToBadRequest() {
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(events.NotSubscribed{Service: "realtime", Type: "member.upserted"}), &invalid), "not a bad request")
	Assert.Equals(13, invalid.Number())
	Assert.Equals("type", invalid.Field)
}

func (test *ErrorMappingServiceTest) TestEventsMapsAnUnknownTypeToBadRequest() {
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(events.UnknownEventType{Type: "dance"}), &invalid), "not a bad request")
	Assert.Equals(6, invalid.Number())
}

func (test *ErrorMappingServiceTest) TestEventsMapsAMemberCallerToForbidden() {
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(events.NotService{}), &forbidden), "not forbidden")
	Assert.Equals(12, forbidden.Number())
}

func (test *ErrorMappingServiceTest) TestEventsMapsAnotherDomainToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(realtime.Missing{}.EventDomain()), &missing), "not missing")
	Assert.Equals(5019, missing.Number())
}

func (test *ErrorMappingServiceTest) TestEventsMapsARefusedTokenToUnauthenticated() {
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(realtime.Unauthenticated{}.TokenRefused()), &refused), "not unauthenticated")
	Assert.Equals(5011, refused.Number())
}

func (test *ErrorMappingServiceTest) TestEventsMapsAStorageFailureToUnavailable() {
	unavailable := &Unavailable{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).EventError(errors.New("down")), &unavailable), "not unavailable")
	Assert.Equals(5024, unavailable.Number())
}

func (test *ErrorMappingServiceTest) TestNoFailureIsNoError() {
	Assert.IsTrue((&ErrorMappingService{}).EventError(nil) == nil, "an error without a failure")
}

func (test *ErrorMappingServiceTest) TestPresenceMapsAnInvalidRequestWithItsFieldAndNumber() {
	invalid := &InvalidRequest{}
	errors.As((&ErrorMappingService{}).PresenceError(realtime.Invalid{}.UnknownState()), &invalid)
	Assert.Equals("state", invalid.Field)
	Assert.Equals(5007, invalid.Number())
	Assert.Equals("must be active, idle or dnd", invalid.Message)
}

func (test *ErrorMappingServiceTest) TestPresenceMapsMissingToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).PresenceError(realtime.Missing{}.Presence()), &missing), "not missing")
	Assert.Equals(5021, missing.Number())
}

func (test *ErrorMappingServiceTest) TestSweepMapsAnotherRealmToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).SweepError(realtime.Missing{}.SweepDomain()), &missing), "not missing")
	Assert.Equals(5020, missing.Number())
}

func (test *ErrorMappingServiceTest) TestSweepMapsAMissingRoleToForbidden() {
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).SweepError(realtime.Forbidden{}.NotAService()), &forbidden), "not forbidden")
	Assert.Equals(5017, forbidden.Number())
}

func (test *ErrorMappingServiceTest) TestSweepKeepsAnInvalidTimeUnavailable() {
	unavailable := &Unavailable{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).SweepError(realtime.Invalid{}.NowNotATime()), &unavailable), "not unavailable")
	Assert.Equals(5024, unavailable.Number())
}

func (test *ErrorMappingServiceTest) TestGatewayMapsAMissingConnectionToNotFound() {
	missing := &NotFound{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).GatewayError(realtime.Missing{}.Connection()), &missing), "not missing")
	Assert.Equals(5023, missing.Number())
}

func (test *ErrorMappingServiceTest) TestTicketsMapAnInvalidRequestToUnavailable() {
	unavailable := &Unavailable{}
	Assert.IsTrue(errors.As((&ErrorMappingService{}).TicketError(realtime.Invalid{}.UnknownKind()), &unavailable), "not unavailable")
	Assert.Equals(5024, unavailable.Number())
}
