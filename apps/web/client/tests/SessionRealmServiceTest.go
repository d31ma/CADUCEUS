import "strings"

// @Test(SessionTestSetup, {"partition":"tests"})
type SessionRealmServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionRealmServiceTest) realm() *SessionRealmService {
	client := NewIdentityClientWith(test.behaviors)
	realm := &SessionRealmService{identity: client}
	realm.Load()
	return realm
}

func (test *SessionRealmServiceTest) TestLoadReadsThePublicConfig() {
	realm := test.realm()
	Assert.Equals(nil, realm.Load())
	Assert.Equals("https://mesail.example.test/", realm.RedirectUri())
}

func (test *SessionRealmServiceTest) TestDomainOfLowersAndTrimsTheAddress() {
	Assert.Equals("example.com", test.realm().DomainOf("  Ana@Example.COM "))
}

func (test *SessionRealmServiceTest) TestDomainOfRefusesAnAddressWithoutDomain() {
	Assert.Equals("", test.realm().DomainOf("ana"))
}

func (test *SessionRealmServiceTest) TestDomainOfRefusesAHostWithoutDot() {
	Assert.Equals("", test.realm().DomainOf("ana@localhost"))
}

func (test *SessionRealmServiceTest) TestDomainOfRefusesSpaces() {
	Assert.Equals("", test.realm().DomainOf("ana ruiz@example.com"))
}

func (test *SessionRealmServiceTest) TestIssuerFillsTheRealmPattern() {
	Assert.Equals("https://sign-in.example.test/realms/example.org", test.realm().Issuer("example.org"))
}

func (test *SessionRealmServiceTest) TestIssuerReplacesAFixedRealm() {
	realm := &SessionRealmService{config: ClientConfig{Issuer: "https://id.example.test/realms/example.com"}}
	Assert.Equals("https://id.example.test/realms/example.org", realm.Issuer("example.org"))
}

func (test *SessionRealmServiceTest) TestIssuerAddsTheRealmToABareHost() {
	realm := &SessionRealmService{config: ClientConfig{Issuer: "https://id.example.test/"}}
	Assert.Equals("https://id.example.test/realms/example.org", realm.Issuer("example.org"))
}

func (test *SessionRealmServiceTest) TestSignInHostNamesTheIssuerHost() {
	Assert.Equals("sign-in.example.test", test.realm().SignInHost("example.com"))
}

func (test *SessionRealmServiceTest) TestSignInHostIsEmptyForABrokenIssuer() {
	realm := &SessionRealmService{config: ClientConfig{Issuer: "%zz"}}
	Assert.Equals("", realm.SignInHost("example.com"))
}

func (test *SessionRealmServiceTest) TestAuthorizeUrlAsksForACodeWithTheChallenge() {
	pending := SessionPending{DomainId: "example.com", Email: "ana@example.com", State: "state-1", Verifier: "verifier-1"}
	address := test.realm().AuthorizeUrl(pending, "challenge-1")
	Assert.IsTrue(strings.HasPrefix(address, "https://sign-in.example.test/realms/example.com/protocol/openid-connect/auth?"), address)
	Assert.IsTrue(strings.Contains(address, "code_challenge=challenge-1&code_challenge_method=S256"), address)
	Assert.IsTrue(strings.Contains(address, "client_id=mesail-web"), address)
	Assert.IsTrue(strings.Contains(address, "state=state-1"), address)
	Assert.IsTrue(strings.Contains(address, "login_hint=ana%40example.com"), address)
	Assert.IsTrue(strings.Contains(address, "scope=openid+email"), address)
	Assert.IsTrue(!strings.Contains(address, "verifier-1"), address)
}

func (test *SessionRealmServiceTest) TestLogoutUrlReturnsToTheSignedOutPage() {
	address := test.realm().LogoutUrl("example.com", "id-1")
	Assert.IsTrue(strings.HasPrefix(address, "https://sign-in.example.test/realms/example.com/protocol/openid-connect/logout?"), address)
	Assert.IsTrue(strings.Contains(address, "post_logout_redirect_uri=https%3A%2F%2Fmesail.example.test%2Fsigned-out"), address)
	Assert.IsTrue(strings.Contains(address, "id_token_hint=id-1"), address)
}

func (test *SessionRealmServiceTest) TestLogoutUrlLeavesOutAnEmptyHint() {
	Assert.IsTrue(!strings.Contains(test.realm().LogoutUrl("example.com", ""), "id_token_hint"))
}
