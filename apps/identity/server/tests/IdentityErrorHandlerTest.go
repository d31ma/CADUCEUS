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
