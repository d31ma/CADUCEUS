// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationTokenServiceTest) token(expiresAtMs float64) *ConversationTokenService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "ana", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com"})
	tokens := &SessionTokenService{identity: NewIdentityClientWith(test.behaviors)}
	tokens.tokens = map[string]SessionTokens{"example.com": {AccessToken: "access-1", ExpiresAtMs: expiresAtMs}}
	return &ConversationTokenService{tokens: tokens, accounts: accounts}
}

func (test *ConversationTokenServiceTest) TestAuthorizationIsABearerHeader() {
	authorization, failure := test.token(100000000).Authorization()
	Assert.Equals(nil, failure)
	Assert.Equals("Bearer access-1", authorization)
}

func (test *ConversationTokenServiceTest) TestAuthorizationFailsWhenTheSessionEnded() {
	_, failure := test.token(0).Authorization()
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
	token.accounts.Add(Me{MemberId: "ana", DomainId: "example.org", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.org"})
	Assert.Equals("example.org", token.DomainId())
	Assert.Equals("ana@example.org", token.MyAddress())
}

func (test *ConversationTokenServiceTest) TestMyAddressIsTheCurrentAccountAddress() {
	Assert.Equals("ana@example.com", test.token(100000000).MyAddress())
}
