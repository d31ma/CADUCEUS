import (
	"errors"
	"net/http"

	"mesail.dev/packages/mailout"
	"mesail.dev/packages/oidc"
)

// @Test(MailOutTestSetup)
type MailOutPartsTest struct {
	behaviors *BehaviorContext
}

func (test *MailOutPartsTest) TestRejectedAnswersInvalidRequestWithTheNumber() {
	answer := (&MailOutErrorHandler{}).Rejected(RejectedEvent{}.EventRule("the event envelope is not valid: eventId must not be empty"))
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS3001]: the event envelope is not valid: eventId must not be empty"}, answer)
}

func (test *MailOutPartsTest) TestRejectedAnswersTheNumberOfASendRequestCase() {
	answer := (&MailOutErrorHandler{}).Rejected(RejectedEvent{}.SendRequest("the email.send.requested field from is missing or not valid", 3010))
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS3010]: the email.send.requested field from is missing or not valid"}, answer)
}

func (test *MailOutPartsTest) TestForbiddenAnswersForbiddenWithTheNumber() {
	answer := (&MailOutErrorHandler{}).Forbidden(&ForbiddenCaller{})
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS3002]: Only a caller with the Service role may deliver events."}, answer)
}

func (test *MailOutPartsTest) TestUnknownAnswersNotFoundWithTheNumber() {
	answer := (&MailOutErrorHandler{}).Unknown(&UnknownDomain{ErrorNumber: 3003})
	Assert.Equals(ApiError{Code: "not_found", Message: "[MS3003]: Not found."}, answer)
}

func (test *MailOutPartsTest) TestUnexpectedAnswersTheUnexpectedErrorNumber() {
	answer := (&MailOutErrorHandler{}).Unexpected(errors.New("the queue store failed"))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS3000]: Something went wrong."}, answer)
}

func (test *MailOutPartsTest) TestAnInstallWideTokenIsAServiceCaller() {
	realm := MailOutTestSetup{}.Realm()
	MailOutTestSetup{}.Use(mailout.Settings{}, "http://127.0.0.1:1")
	request, _ := http.NewRequest(http.MethodPost, "/api/v1/mail-out/jobs/deliver", nil)
	request.Header.Set("Authorization", "Bearer "+MailOutTestSetup{}.ServiceToken(realm.ForDomain(oidc.InstallWide{}.DomainId())))
	identity := (&TokenIdentityProvider{}).Identify(request)
	Assert.IsTrue(identity != nil, "the token is accepted")
	Assert.Equals([]string{"Service"}, identity.Roles)
	Assert.Equals(oidc.InstallWide{}.DomainId()+"/service-account", identity.User)
}
