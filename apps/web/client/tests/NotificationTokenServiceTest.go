// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationTokenServiceTest) sendWith(access *NotificationTokenService, failures []error) ([]string, error) {
	sent := []string{}
	_, failure := access.Send(func(authorization string) (BehaviorValue, error) {
		next := failures[len(sent)]
		sent = append(sent, authorization)
		return "answer", next
	})
	return sent, failure
}

func (test *NotificationTokenServiceTest) TestSendUsesTheCurrentDomainToken() {
	access := NotificationTestSetup{}.access(test.behaviors, "example.com")
	sent, failure := test.sendWith(access, []error{nil})
	Assert.Equals(nil, failure)
	Assert.Equals([]string{"Bearer access-1"}, sent)
	Assert.Equals("example.com", access.DomainId())
	Assert.Equals("Ana Ruiz", access.Account().DisplayName)
}

func (test *NotificationTokenServiceTest) TestSendRenewsOnceAfterAnEndedToken() {
	access := NotificationTestSetup{}.access(test.behaviors, "example.com")
	sent, failure := test.sendWith(access, []error{&BehaviorError{Status: 401}, nil})
	Assert.Equals(nil, failure)
	Assert.Equals([]string{"Bearer access-1", "Bearer access-2"}, sent)
}

func (test *NotificationTokenServiceTest) TestSecondEndedTokenShowsTheSessionProblem() {
	access := NotificationTestSetup{}.access(test.behaviors, "example.com")
	sent, failure := test.sendWith(access, []error{&BehaviorError{Status: 401}, &BehaviorError{Status: 401}})
	Assert.Equals(2, len(sent))
	Assert.Equals("session", (&NotificationProblemService{}).OfFailure(failure))
}

func (test *NotificationTokenServiceTest) TestFailedRenewalSendsNothingMore() {
	access := NotificationTestSetup{}.access(test.behaviors, "broken.example")
	sent, failure := test.sendWith(access, []error{&BehaviorError{Status: 401}})
	Assert.Equals(1, len(sent))
	Assert.IsTrue(failure != nil)
}

func (test *NotificationTokenServiceTest) TestOtherFailuresAreNotRetried() {
	access := NotificationTestSetup{}.access(test.behaviors, "example.com")
	sent, failure := test.sendWith(access, []error{&BehaviorError{Status: 503}})
	Assert.Equals(1, len(sent))
	Assert.Equals("error", (&NotificationProblemService{}).OfFailure(failure))
}

func (test *NotificationTokenServiceTest) TestSendWaitsWhileNobodyIsSignedIn() {
	sent, failure := test.sendWith(NotificationTestSetup{}.signedOutAccess(test.behaviors), []error{nil})
	Assert.Equals(0, len(sent))
	Assert.Equals("waiting", (&NotificationProblemService{}).OfFailure(failure))
}
