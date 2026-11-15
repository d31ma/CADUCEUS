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
	known := (&FailureService{}).Known(errors.Join(errors.New("plain"), core.NotFound{}.NoMessage(), core.Conflict{}.MessageDeleted()))
	Assert.Equals(&NotFound{Message: "no such message", ErrorNumber: 1539}, known)
}

func (test *FailureServiceTest) TestAnUnavailableFailureDropsItsKnownCause() {
	failure := errors.Join(core.Unavailable{}.Store(core.Conflict{}.MessageDeleted()), core.NotFound{}.NoMessage())
	known := (&FailureService{}).Known(failure)
	Assert.Equals(&Unavailable{Message: "the message store is not available", ErrorNumber: 1551}, known)
}

func (test *FailureServiceTest) TestAnUnknownFailureBecomesUnavailable() {
	known := (&FailureService{}).Known(errors.New("socket closed"))
	Assert.Equals(&Unavailable{Message: "the messaging service is not available", ErrorNumber: 1507}, known)
}

func (test *FailureServiceTest) TestEventRefusalsAnswerTheirKitNumbers() {
	service := &FailureService{}
	Assert.Equals(&InvalidRequest{Message: events.InvalidEnvelope{}.EmptyPayload().Error(), ErrorNumber: 9}, service.Known(events.InvalidEnvelope{}.EmptyPayload()))
	Assert.Equals(&InvalidRequest{Message: events.NotSubscribed{Type: "x"}.Error(), ErrorNumber: 13}, service.Known(events.NotSubscribed{Type: "x"}))
	Assert.Equals(&InvalidRequest{Message: events.UnknownEventType{Type: "x"}.Error(), ErrorNumber: 6}, service.Known(events.UnknownEventType{Type: "x"}))
	Assert.Equals(&InvalidRequest{Message: inbound.UnhandledEvent{Type: "x"}.Error(), ErrorNumber: 1511}, service.Known(inbound.UnhandledEvent{Type: "x"}))
}

func (test *FailureServiceTest) TestANonServiceCallerIsForbidden() {
	Assert.Equals(&Forbidden{Message: events.NotService{}.Error(), ErrorNumber: 12}, (&FailureService{}).Known(events.NotService{}))
}

func (test *FailureServiceTest) TestCoreFailuresKeepTheirNumbers() {
	service := &FailureService{}
	Assert.Equals(&Forbidden{Message: "only the author can change this message", ErrorNumber: 1537}, service.Known(core.Forbidden{}.NotTheAuthor()))
	Assert.Equals(&InvalidRequest{Message: "an email needs a subject", Field: "subject", ErrorNumber: 1521}, service.Known(core.InvalidRequest{}.MissingSubject()))
	Assert.Equals(&Conflict{Message: "the channel is archived and read-only", ErrorNumber: 1546}, service.Known(core.Conflict{}.ChannelArchived()))
	Assert.Equals(&ChangedRecipients{Message: "the recipients changed", Addresses: []string{"a"}, ErrorNumber: 1547}, service.Known(core.RecipientsChanged{}.Of([]string{"a"})))
	Assert.Equals(&ChatNotAllowed{Message: "chat is not allowed for a", Address: "a", ErrorNumber: 1550}, service.Known(core.ChatNotAllowed{}.For("a")))
	Assert.Equals(&TooLarge{Message: "the message is too large to store; send a shorter body", ErrorNumber: 1548}, service.Known(core.TooLarge{}.StoredMessage()))
	Assert.Equals(&NotFound{Message: "no such conversation", ErrorNumber: 1538}, service.Known(core.NotFound{}.NoConversation()))
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
