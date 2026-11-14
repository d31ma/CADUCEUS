import (
	"strings"

	"mesail.dev/packages/core"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type MessagingErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *MessagingErrorHandlerTest) TestInvalidKeepsTheFieldAndTheNumber() {
	answer := (&MessagingErrorHandler{}).Invalid(InvalidRequest{}.BadLimit())
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS1506]: limit must be a whole number from 1 to 100", Field: "limit"}, answer)
}

func (test *MessagingErrorHandlerTest) TestAccessRefusalsHaveTheirCodesAndNumbers() {
	handler := &MessagingErrorHandler{}
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "[MS1501]: the access token was refused"}, handler.Unauthenticated(Unauthenticated{}.TokenRefused()))
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS1502]: only members of a domain use its conversations"}, handler.Forbidden(Forbidden{}.NotADomainMember()))
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS1503]: only a caller with the Service role may use this route"}, handler.Forbidden(Forbidden{}.NotAService()))
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS1504]: the outbox is managed with an install-wide token only"}, handler.Forbidden(Forbidden{}.NotInstallWide()))
	Assert.Equals(ApiError{Code: "not_found", Message: "[MS1505]: no such domain"}, handler.Missing(NotFound{}.OtherDomain()))
}

func (test *MessagingErrorHandlerTest) TestEventRefusalsHaveTheirNumbers() {
	handler := &MessagingErrorHandler{}
	Assert.Equals("[MS1508]: bad envelope", handler.Invalid(InvalidRequest{}.BadEnvelope("bad envelope")).Message)
	Assert.Equals("[MS1509]: not consumed", handler.Invalid(InvalidRequest{}.NotSubscribed("not consumed")).Message)
	Assert.Equals("[MS1510]: unknown type", handler.Invalid(InvalidRequest{}.UnknownEventType("unknown type")).Message)
	Assert.Equals("[MS1511]: no handler", handler.Invalid(InvalidRequest{}.UnhandledEvent("no handler")).Message)
	Assert.Equals("[MS1512]: not a service", handler.Forbidden(Forbidden{}.EventsNeedService("not a service")).Message)
}

func (test *MessagingErrorHandlerTest) TestConflictsHaveTheirCodesAndNumbers() {
	handler := &MessagingErrorHandler{}
	conflict := (&FailureService{}).Known(core.Conflict{}.ChannelArchived()).(*Conflict)
	Assert.Equals(ApiError{Code: "conflict", Message: "[MS1546]: the channel is archived and read-only"}, handler.Conflict(conflict))
	changed := handler.RecipientsChanged(ChangedRecipients{}.Of([]string{"bo@example.com"}, 1547))
	Assert.Equals(RecipientsChanged{Code: "recipients_changed", Message: "[MS1547]: the recipients changed", Addresses: []string{"bo@example.com"}}, changed)
}

func (test *MessagingErrorHandlerTest) TestRecipientsChangedNeverAnswersNullAddresses() {
	changed := (&MessagingErrorHandler{}).RecipientsChanged(ChangedRecipients{}.Of(nil, 1547))
	Assert.Equals([]string{}, changed.Addresses)
}

func (test *MessagingErrorHandlerTest) TestRecipientsChangedNamesNoAddressInTheMessage() {
	changed := (&MessagingErrorHandler{}).RecipientsChanged(ChangedRecipients{}.Of([]string{"bo@example.com"}, 1547))
	Assert.IsTrue(!strings.Contains(changed.Message, "bo@example.com"), "the message must not list addresses")
}

func (test *MessagingErrorHandlerTest) TestChatNotAllowedNamesTheAddress() {
	failure := (&FailureService{}).Known(core.ChatNotAllowed{}.For("carol@partner.org")).(*ChatNotAllowed)
	answer := (&MessagingErrorHandler{}).ChatNotAllowed(failure)
	Assert.Equals(ApiError{Code: "chat_not_allowed", Message: "[MS1550]: chat is not allowed for carol@partner.org", Field: "carol@partner.org"}, answer)
}

func (test *MessagingErrorHandlerTest) TestTooLargeHasItsCodeAndNumber() {
	failure := (&FailureService{}).Known(core.TooLarge{}.AttachmentTotal()).(*TooLarge)
	answer := (&MessagingErrorHandler{}).TooLarge(failure)
	Assert.Equals(ApiError{Code: "too_large", Message: "[MS1549]: the attachments of a message may have at most 50 MiB together"}, answer)
}

func (test *MessagingErrorHandlerTest) TestUnavailableHasItsCodeAndNumber() {
	answer := (&MessagingErrorHandler{}).Unavailable(Unavailable{}.ServiceFailed())
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS1507]: the messaging service is not available"}, answer)
}

func (test *MessagingErrorHandlerTest) TestAnUnexpectedErrorAnswersTheFirstNumber() {
	answer := (&MessagingErrorHandler{}).Unexpected(core.Unavailable{}.Store(nil))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS1500]: Something went wrong."}, answer)
}

func (test *MessagingErrorHandlerTest) TestEveryAnswerNumberIsInTheMessagingBlock() {
	Assert.IsTrue(Unavailable{}.Unexpected().Number() == 1500, "the catch-all is the first number")
	Assert.IsTrue(InvalidRequest{}.UnhandledEvent("x").Number() < 2000, "the app numbers stay below MS2000")
	Assert.IsTrue(Forbidden{}.NotADomainMember().Number() > 1500, "the app numbers stay above MS1500")
}
