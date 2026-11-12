import (
	"net/http"

	"mesail.dev/packages/mailout"
	"mesail.dev/packages/oidc"
)

// @Test(MailOutTestSetup)
type MailOutPartsTest struct {
	behaviors *BehaviorContext
}

func (test *MailOutPartsTest) TestErrorHandlerAnswersWithApiErrors() {
	handler := &MailOutErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad"}, handler.Rejected(&RejectedEvent{Reason: "bad"}))
	Assert.Equals(ApiError{Code: "forbidden", Message: "only a caller with the Service role may deliver events"}, handler.Forbidden(&ForbiddenCaller{}))
	Assert.Equals(ApiError{Code: "not_found", Message: "not found"}, handler.Unknown(&UnknownDomain{}))
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
