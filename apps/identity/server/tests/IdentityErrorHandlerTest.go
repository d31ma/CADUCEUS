import "errors"

// @Test(OneRealmTestSetup, {"partition":"tests"})
type IdentityErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *IdentityErrorHandlerTest) handler() *IdentityErrorHandler {
	return &IdentityErrorHandler{}
}

func (test *IdentityErrorHandlerTest) TestUnexpectedErrorIsTheFirstNumber() {
	answer := test.handler().Unexpected(errors.New("disk full at /var/lib"))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS0500]: Something went wrong.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestSignInFailedIsNumbered() {
	answer := test.handler().Unauthenticated(Unauthenticated{}.SignInFailed())
	Assert.Equals("unauthenticated", answer.Code)
	Assert.Equals("[MS0501]: Sign-in failed. Check your details and try again.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestRefusedTokenIsNumbered() {
	answer := test.handler().Unauthenticated(Unauthenticated{}.TokenRefused())
	Assert.Equals("unauthenticated", answer.Code)
	Assert.Equals("[MS0502]: Your sign-in has ended. Sign in again.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestLockedAccountIsNumbered() {
	answer := test.handler().RateLimited(RateLimited{}.Locked())
	Assert.Equals("rate_limited", answer.Code)
	Assert.Equals("[MS0503]: Too many sign-in attempts. Wait a few minutes.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestSignInUnavailableIsNumbered() {
	answer := test.handler().Unavailable(Unavailable{}.SignIn())
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS0504]: Sign-in is not available now. Try again later.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestNotAMemberIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.NotAMember())
	Assert.Equals("not_a_member", answer.Code)
	Assert.Equals("[MS0505]: You are not a member of that domain.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestNoDomainIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.NoDomain())
	Assert.Equals("no_domain", answer.Code)
	Assert.Equals("[MS0506]: This account may not open that domain.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestAccessRevokedIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.AccessRevoked())
	Assert.Equals("access_revoked", answer.Code)
	Assert.Equals("[MS0507]: Your access was removed. Sign in again.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestPasswordPolicyNamesTheRule() {
	answer := test.handler().BadRequest(BadRequest{}.PasswordPolicy("Invalid password: minimum length 12."))
	Assert.Equals("password_policy", answer.Code)
	Assert.Equals("newPassword", answer.Field)
	Assert.Equals("[MS0508]: Invalid password: minimum length 12.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestInvalidCodeNamesTheCodeField() {
	answer := test.handler().BadRequest(BadRequest{}.InvalidCode())
	Assert.Equals("invalid_code", answer.Code)
	Assert.Equals("code", answer.Field)
	Assert.Equals("[MS0509]: The 2-step code is not right.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestLabelExistsIsNumbered() {
	answer := test.handler().Conflict(Conflict{}.LabelExists())
	Assert.Equals("label_exists", answer.Code)
	Assert.Equals("[MS0510]: This passkey name is already used.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestTooManyPasskeysIsNumbered() {
	answer := test.handler().Conflict(Conflict{}.TooManyPasskeys())
	Assert.Equals("too_many", answer.Code)
	Assert.Equals("[MS0511]: You have the most passkeys allowed. Remove one first.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestSetupExpiredIsNumbered() {
	answer := test.handler().BadRequest(BadRequest{}.SetupExpired())
	Assert.Equals("setup_expired", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS0512]: The passkey setup has expired. Start again.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestInvalidPasskeyIsNumbered() {
	answer := test.handler().BadRequest(BadRequest{}.InvalidPasskey())
	Assert.Equals("invalid_passkey", answer.Code)
	Assert.Equals("[MS0513]: The passkey could not be added. Start again.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestInvalidLabelNamesTheLabelField() {
	answer := test.handler().BadRequest(BadRequest{}.InvalidLabel())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("label", answer.Field)
	Assert.Equals("[MS0514]: A passkey name is 1 to 64 characters.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestMissingPasskeyIsNumbered() {
	answer := test.handler().NotFound(NotFound{}.Passkey())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("[MS0515]: That passkey was not found.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestMissingDomainIsNumbered() {
	answer := test.handler().NotFound(NotFound{}.Domain())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("[MS0516]: That domain was not found.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestInvalidDomainNamesTheDomainField() {
	answer := test.handler().BadRequest(BadRequest{}.InvalidDomain())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("domainId", answer.Field)
	Assert.Equals("[MS0517]: The domain id must be a domain name or *.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestRefusedClientIsNumbered() {
	answer := test.handler().Unauthenticated(Unauthenticated{}.ClientRefused())
	Assert.Equals("unauthenticated", answer.Code)
	Assert.Equals("[MS0518]: The client id or client secret is not right.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestServiceTokensUnavailableIsNumbered() {
	answer := test.handler().Unavailable(Unavailable{}.ServiceTokens())
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS0519]: Service tokens are not available now. Try again later.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestSigningKeysUnavailableIsNumbered() {
	answer := test.handler().Unavailable(Unavailable{}.SigningKeys())
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS0520]: The signing keys are not available now. Try again later.", answer.Message)
}
