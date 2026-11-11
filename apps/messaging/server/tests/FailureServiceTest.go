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
	Assert.IsTrue(known == error(missing), "want the NotFound itself")
}

func (test *FailureServiceTest) TestAnUnavailableFailureDropsItsKnownCause() {
	failure := errors.Join(&core.Unavailable{Message: "the store is down", Cause: &core.Conflict{Message: "x"}}, &core.NotFound{Message: "no"})
	known := (&FailureService{}).Known(failure)
	Assert.Equals(&core.Unavailable{Message: "the store is down"}, known)
}

func (test *FailureServiceTest) TestAnUnknownFailureBecomesUnavailable() {
	cause := errors.New("socket closed")
	known := (&FailureService{}).Known(cause)
	Assert.Equals(&core.Unavailable{Message: "the messaging service is not available", Cause: cause}, known)
}

func (test *FailureServiceTest) TestEventFailuresKeepTheirValueTypes() {
	service := &FailureService{}
	Assert.Equals(events.NotService{}, service.Known(events.NotService{}))
	Assert.Equals(events.InvalidEnvelope{Reason: "x"}, service.Known(events.InvalidEnvelope{Reason: "x"}))
	Assert.Equals(events.NotSubscribed{Type: "x"}, service.Known(events.NotSubscribed{Type: "x"}))
	Assert.Equals(events.UnknownEventType{Type: "x"}, service.Known(events.UnknownEventType{Type: "x"}))
	Assert.Equals(inbound.UnhandledEvent{Type: "x"}, service.Known(inbound.UnhandledEvent{Type: "x"}))
}

func (test *FailureServiceTest) TestChatFailuresKeepTheirPointers() {
	service := &FailureService{}
	unauthenticated := &Unauthenticated{Message: "no"}
	forbidden := &core.Forbidden{Message: "no"}
	invalid := &core.InvalidRequest{Field: "body"}
	conflict := &core.Conflict{Message: "archived"}
	changed := &core.RecipientsChanged{Addresses: []string{"a"}}
	notAllowed := &core.ChatNotAllowed{Address: "a"}
	Assert.IsTrue(service.Known(unauthenticated) == error(unauthenticated), "unauthenticated")
	Assert.IsTrue(service.Known(forbidden) == error(forbidden), "forbidden")
	Assert.IsTrue(service.Known(invalid) == error(invalid), "invalid")
	Assert.IsTrue(service.Known(conflict) == error(conflict), "conflict")
	Assert.IsTrue(service.Known(changed) == error(changed), "recipients changed")
	Assert.IsTrue(service.Known(notAllowed) == error(notAllowed), "chat not allowed")
}
