// @Test(OneRealmTestSetup, {"partition":"tests"})
type IdentityErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *IdentityErrorHandlerTest) TestUnauthenticatedIsUnauthenticated() {
	answer := (&IdentityErrorHandler{}).Unauthenticated(&Unauthenticated{})
	Assert.Equals("unauthenticated", answer.Code)
}

func (test *IdentityErrorHandlerTest) TestRateLimitedIsRateLimited() {
	answer := (&IdentityErrorHandler{}).RateLimited(&RateLimited{})
	Assert.Equals("rate_limited", answer.Code)
}

func (test *IdentityErrorHandlerTest) TestUnavailableIsUnavailable() {
	answer := (&IdentityErrorHandler{}).Unavailable(&Unavailable{})
	Assert.Equals("unavailable", answer.Code)
}

func (test *IdentityErrorHandlerTest) TestPasswordPolicyNamesTheRule() {
	answer := (&IdentityErrorHandler{}).PasswordPolicy(&PasswordPolicy{Rule: "Invalid password: minimum length 12."})
	Assert.Equals("password_policy", answer.Code)
	Assert.Equals("Invalid password: minimum length 12.", answer.Message)
}

func (test *IdentityErrorHandlerTest) TestInvalidCodeIsInvalidCode() {
	answer := (&IdentityErrorHandler{}).InvalidCode(&InvalidCode{})
	Assert.Equals("invalid_code", answer.Code)
}

func (test *IdentityErrorHandlerTest) TestForbiddenKeepsItsCode() {
	answer := (&IdentityErrorHandler{}).Forbidden(&Forbidden{Code: "not_a_member"})
	Assert.Equals("not_a_member", answer.Code)
}

func (test *IdentityErrorHandlerTest) TestNotFoundIsNotFound() {
	answer := (&IdentityErrorHandler{}).NotFound(&NotFound{})
	Assert.Equals("not_found", answer.Code)
}

func (test *IdentityErrorHandlerTest) TestInvalidRequestNamesTheDomainField() {
	answer := (&IdentityErrorHandler{}).InvalidRequest(&InvalidRequest{})
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("domainId", answer.Field)
}
