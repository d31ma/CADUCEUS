// @Test(KeycloakTestSetup, {"partition":"tests"})
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
