import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/inbound"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type MessagingErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *MessagingErrorHandlerTest) TestInvalidKeepsTheField() {
	answer := (&MessagingErrorHandler{}).Invalid(&core.InvalidRequest{Message: "bad", Field: "body"})
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad", Field: "body"}, answer)
}

func (test *MessagingErrorHandlerTest) TestEventRefusalsAreInvalidRequests() {
	handler := &MessagingErrorHandler{}
	Assert.Equals("invalid_request", handler.InvalidEnvelope(events.InvalidEnvelope{Reason: "no id"}).Code)
	Assert.Equals("invalid_request", handler.NotSubscribed(events.NotSubscribed{Service: "messaging", Type: "x"}).Code)
	Assert.Equals("invalid_request", handler.UnknownEventType(events.UnknownEventType{Type: "x"}).Code)
	Assert.Equals("invalid_request", handler.UnhandledEvent(inbound.UnhandledEvent{Type: "x"}).Code)
}

func (test *MessagingErrorHandlerTest) TestAccessRefusalsHaveTheirCodes() {
	handler := &MessagingErrorHandler{}
	failure := &Unauthenticated{Message: "the access token was refused"}
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "the access token was refused"}, handler.Unauthenticated(failure))
	Assert.Equals("the access token was refused", failure.Error())
	Assert.Equals("forbidden", handler.Forbidden(&core.Forbidden{Message: "no"}).Code)
	Assert.Equals("forbidden", handler.NotService(events.NotService{}).Code)
	Assert.Equals("not_found", handler.Missing(&core.NotFound{Message: "no"}).Code)
}

func (test *MessagingErrorHandlerTest) TestConflictsHaveTheirCodes() {
	handler := &MessagingErrorHandler{}
	Assert.Equals("conflict", handler.Conflict(&core.Conflict{Message: "archived"}).Code)
	changed := handler.RecipientsChanged(&core.RecipientsChanged{Addresses: []string{"bo@example.com"}})
	Assert.Equals(RecipientsChanged{Code: "recipients_changed", Message: "the recipients changed: bo@example.com", Addresses: []string{"bo@example.com"}}, changed)
}

func (test *MessagingErrorHandlerTest) TestRecipientsChangedNeverAnswersNullAddresses() {
	changed := (&MessagingErrorHandler{}).RecipientsChanged(&core.RecipientsChanged{})
	Assert.Equals([]string{}, changed.Addresses)
}

func (test *MessagingErrorHandlerTest) TestChatNotAllowedNamesTheAddress() {
	answer := (&MessagingErrorHandler{}).ChatNotAllowed(&core.ChatNotAllowed{Address: "carol@partner.org"})
	Assert.Equals(ApiError{Code: "chat_not_allowed", Message: "chat is not allowed for carol@partner.org", Field: "carol@partner.org"}, answer)
}

func (test *MessagingErrorHandlerTest) TestUnavailableHidesTheCause() {
	answer := (&MessagingErrorHandler{}).Unavailable(&core.Unavailable{Message: "the message store is not available", Cause: &core.Conflict{Message: "disk full"}})
	Assert.Equals(ApiError{Code: "unavailable", Message: "the message store is not available"}, answer)
}
