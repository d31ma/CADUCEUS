// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationTokenServiceTest) token(expiresAtMs float64) *ConversationTokenService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	accounts := &SessionAccountService{}
	accounts.Apply(Session{Me: Me{Username: "ana", MemberId: "ana", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com"}, Domains: []DomainChoice{{DomainId: "example.com", Default: true}}})
	tokens := &SessionTokenService{identity: NewIdentityClientWith(test.behaviors)}
	tokens.key = "key-1"
	tokens.tokens = SessionTokens{AccessToken: "access-1", ExpiresAtMs: expiresAtMs}
	return &ConversationTokenService{session: &SessionService{tokens: tokens, accounts: accounts}, accounts: accounts}
}

func (test *ConversationTokenServiceTest) TestAuthorizationIsABearerHeader() {
	authorization, failure := test.token(100000000).authorization()
	Assert.Equals(nil, failure)
	Assert.Equals("Bearer access-1", authorization)
}

func (test *ConversationTokenServiceTest) TestAuthorizationFailsWhenTheSessionEnded() {
	_, failure := test.token(0).authorization()
	Assert.IsTrue(failure != nil)
}

func (test *ConversationTokenServiceTest) TestAccessTokenIsTheRawToken() {
	accessToken, failure := test.token(100000000).AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("access-1", accessToken)
}

func (test *ConversationTokenServiceTest) TestAccessTokenFailsWhenTheSessionEnded() {
	_, failure := test.token(0).AccessToken()
	Assert.IsTrue(failure != nil)
}

func (test *ConversationTokenServiceTest) TestDomainIdIsTheCurrentDomain() {
	Assert.Equals("example.com", test.token(100000000).DomainId())
}

func (test *ConversationTokenServiceTest) TestDomainIdFollowsASwitch() {
	token := test.token(100000000)
	token.accounts.Apply(Session{Me: Me{Username: "ana", MemberId: "ana", DomainId: "example.org", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.org"}, Domains: []DomainChoice{{DomainId: "example.org", Default: true}}})
	Assert.Equals("example.org", token.DomainId())
	Assert.Equals("ana@example.org", token.MyAddress())
}

func (test *ConversationTokenServiceTest) TestMyAddressIsTheCurrentAccountAddress() {
	Assert.Equals("ana@example.com", test.token(100000000).MyAddress())
}

func (test *ConversationTokenServiceTest) TestWithRenewalCallsOnceWhenTheCallWorks() {
	calls := []string{}
	failure := test.token(100000000).WithRenewal(func(authorization string) error {
		calls = append(calls, authorization)
		return nil
	})
	Assert.Equals(nil, failure)
	Assert.Equals([]string{"Bearer access-1"}, calls)
}

func (test *ConversationTokenServiceTest) TestWithRenewalRenewsOnceAfterAnUnauthorizedAnswer() {
	calls := []string{}
	failure := test.token(100000000).WithRenewal(func(authorization string) error {
		calls = append(calls, authorization)
		return &BehaviorError{Code: "HTTP_STATUS", Status: 401}
	})
	Assert.IsTrue(failure != nil)
	Assert.Equals(2, len(calls))
}

func (test *ConversationTokenServiceTest) TestWithRenewalKeepsOtherFailures() {
	calls := []string{}
	failure := test.token(100000000).WithRenewal(func(authorization string) error {
		calls = append(calls, authorization)
		return &BehaviorError{Code: "HTTP_STATUS", Status: 503}
	})
	Assert.IsTrue(failure != nil)
	Assert.Equals(1, len(calls))
}

func (test *ConversationTokenServiceTest) TestWithRenewalStopsWhenTheSessionEnded() {
	calls := []string{}
	failure := test.token(0).WithRenewal(func(authorization string) error {
		calls = append(calls, authorization)
		return nil
	})
	Assert.IsTrue(failure != nil)
	Assert.Equals(0, len(calls))
}
