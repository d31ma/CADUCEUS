// @Test(MessagingTestSetup, {"partition":"tests"})
type MessagingErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *MessagingErrorHandlerTest) TestInvalidKeepsTheField() {
	answer := (&MessagingErrorHandler{}).Invalid(&InvalidRequest{Message: "bad", Field: "body"})
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad", Field: "body"}, answer)
	Assert.Equals("bad", (&InvalidRequest{Message: "bad"}).Error())
}

func (test *MessagingErrorHandlerTest) TestAccessRefusalsHaveTheirCodes() {
	handler := &MessagingErrorHandler{}
	failure := &Unauthenticated{Message: "the access token was refused"}
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "the access token was refused"}, handler.Unauthenticated(failure))
	Assert.Equals("the access token was refused", failure.Error())
	Assert.Equals(ApiError{Code: "forbidden", Message: "no"}, handler.Forbidden(&Forbidden{Message: "no"}))
	Assert.Equals("no", (&Forbidden{Message: "no"}).Error())
	Assert.Equals(ApiError{Code: "not_found", Message: "no"}, handler.Missing(&NotFound{Message: "no"}))
	Assert.Equals("no", (&NotFound{Message: "no"}).Error())
}

func (test *MessagingErrorHandlerTest) TestConflictsHaveTheirCodes() {
	handler := &MessagingErrorHandler{}
	Assert.Equals(ApiError{Code: "conflict", Message: "archived"}, handler.Conflict(&Conflict{Message: "archived"}))
	Assert.Equals("archived", (&Conflict{Message: "archived"}).Error())
	failure := &ChangedRecipients{Message: "the recipients changed: bo@example.com", Addresses: []string{"bo@example.com"}}
	changed := handler.RecipientsChanged(failure)
	Assert.Equals(RecipientsChanged{Code: "recipients_changed", Message: "the recipients changed: bo@example.com", Addresses: []string{"bo@example.com"}}, changed)
	Assert.Equals("the recipients changed: bo@example.com", failure.Error())
}

func (test *MessagingErrorHandlerTest) TestRecipientsChangedNeverAnswersNullAddresses() {
	changed := (&MessagingErrorHandler{}).RecipientsChanged(&ChangedRecipients{})
	Assert.Equals([]string{}, changed.Addresses)
}

func (test *MessagingErrorHandlerTest) TestChatNotAllowedNamesTheAddress() {
	failure := &ChatNotAllowed{Message: "chat is not allowed for carol@partner.org", Address: "carol@partner.org"}
	answer := (&MessagingErrorHandler{}).ChatNotAllowed(failure)
	Assert.Equals(ApiError{Code: "chat_not_allowed", Message: "chat is not allowed for carol@partner.org", Field: "carol@partner.org"}, answer)
	Assert.Equals("chat is not allowed for carol@partner.org", failure.Error())
}

func (test *MessagingErrorHandlerTest) TestUnavailableHasItsCode() {
	failure := &Unavailable{Message: "the message store is not available"}
	answer := (&MessagingErrorHandler{}).Unavailable(failure)
	Assert.Equals(ApiError{Code: "unavailable", Message: "the message store is not available"}, answer)
	Assert.Equals("the message store is not available", failure.Error())
}
