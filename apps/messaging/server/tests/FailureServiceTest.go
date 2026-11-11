import (
	"errors"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/inbound"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type FailureServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FailureServiceTest) TestNoFailureStaysNone() {
	Assert.IsTrue((&FailureService{}).Known(nil) == nil, "nil must stay nil")
}

func (test *FailureServiceTest) TestAJoinedFailureBecomesItsFirstKnownFailure() {
	missing := &core.NotFound{Message: "no such message"}
	known := (&FailureService{}).Known(errors.Join(errors.New("plain"), missing, &core.Conflict{Message: "later"}))
	Assert.Equals(&NotFound{Message: "no such message"}, known)
}

func (test *FailureServiceTest) TestAnUnavailableFailureDropsItsKnownCause() {
	failure := errors.Join(&core.Unavailable{Message: "the store is down", Cause: &core.Conflict{Message: "x"}}, &core.NotFound{Message: "no"})
	known := (&FailureService{}).Known(failure)
	Assert.Equals(&Unavailable{Message: "the store is down"}, known)
}

func (test *FailureServiceTest) TestAnUnknownFailureBecomesUnavailable() {
	known := (&FailureService{}).Known(errors.New("socket closed"))
	Assert.Equals(&Unavailable{Message: "the messaging service is not available"}, known)
}

func (test *FailureServiceTest) TestEventRefusalsBecomeInvalidRequests() {
	service := &FailureService{}
	Assert.Equals(&InvalidRequest{Message: events.InvalidEnvelope{Reason: "x"}.Error()}, service.Known(events.InvalidEnvelope{Reason: "x"}))
	Assert.Equals(&InvalidRequest{Message: events.NotSubscribed{Type: "x"}.Error()}, service.Known(events.NotSubscribed{Type: "x"}))
	Assert.Equals(&InvalidRequest{Message: events.UnknownEventType{Type: "x"}.Error()}, service.Known(events.UnknownEventType{Type: "x"}))
	Assert.Equals(&InvalidRequest{Message: inbound.UnhandledEvent{Type: "x"}.Error()}, service.Known(inbound.UnhandledEvent{Type: "x"}))
}

func (test *FailureServiceTest) TestANonServiceCallerIsForbidden() {
	Assert.Equals(&Forbidden{Message: events.NotService{}.Error()}, (&FailureService{}).Known(events.NotService{}))
}

func (test *FailureServiceTest) TestCoreFailuresBecomeAnswerFailures() {
	service := &FailureService{}
	Assert.Equals(&Forbidden{Message: "no"}, service.Known(&core.Forbidden{Message: "no"}))
	Assert.Equals(&InvalidRequest{Message: "bad", Field: "body"}, service.Known(&core.InvalidRequest{Message: "bad", Field: "body"}))
	Assert.Equals(&Conflict{Message: "archived"}, service.Known(&core.Conflict{Message: "archived"}))
	Assert.Equals(&ChangedRecipients{Message: "the recipients changed: a", Addresses: []string{"a"}}, service.Known(&core.RecipientsChanged{Addresses: []string{"a"}}))
	Assert.Equals(&ChatNotAllowed{Message: "chat is not allowed for a", Address: "a"}, service.Known(&core.ChatNotAllowed{Address: "a"}))
	Assert.Equals(&TooLarge{Message: "big"}, service.Known(&core.TooLarge{Message: "big"}))
}

func (test *FailureServiceTest) TestAnswerFailuresKeepTheirPointers() {
	service := &FailureService{}
	unauthenticated := &Unauthenticated{Message: "no"}
	forbidden := &Forbidden{Message: "no"}
	missing := &NotFound{Message: "no"}
	invalid := &InvalidRequest{Field: "limit"}
	tooLarge := &TooLarge{Message: "big"}
	Assert.IsTrue(service.Known(tooLarge) == error(tooLarge), "too large")
	Assert.IsTrue(service.Known(unauthenticated) == error(unauthenticated), "unauthenticated")
	Assert.IsTrue(service.Known(forbidden) == error(forbidden), "forbidden")
	Assert.IsTrue(service.Known(missing) == error(missing), "not found")
	Assert.IsTrue(service.Known(invalid) == error(invalid), "invalid")
}
