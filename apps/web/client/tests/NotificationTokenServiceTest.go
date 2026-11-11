// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationTokenServiceTest) TestAuthorizationUsesTheCurrentDomain() {
	access := NotificationTestSetup{}.access("example.com")
	authorization, failure := access.Authorization()
	Assert.Equals(nil, failure)
	Assert.Equals("Bearer access-1", authorization)
	Assert.Equals("example.com", access.DomainId())
	Assert.Equals("Ana Ruiz", access.Account().DisplayName)
}

func (test *NotificationTokenServiceTest) TestAuthorizationWaitsWhileNobodyIsSignedIn() {
	_, failure := NotificationTestSetup{}.signedOutAccess().Authorization()
	Assert.Equals("waiting", (&NotificationProblemService{}).OfFailure(failure))
}
