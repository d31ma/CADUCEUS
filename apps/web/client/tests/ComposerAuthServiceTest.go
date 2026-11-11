// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerAuthServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerAuthServiceTest) TestAuthorizationIsABearerToken() {
	authorization, failure := ComposerTestSetup{}.Auth(test.behaviors).Authorization()
	Assert.Equals(nil, failure)
	Assert.Equals("Bearer access-1", authorization)
}

func (test *ComposerAuthServiceTest) TestDomainIdIsTheCurrentDomain() {
	Assert.Equals("example.com", ComposerTestSetup{}.Auth(test.behaviors).DomainId())
}

func (test *ComposerAuthServiceTest) TestAddressIsTheSenderAddress() {
	Assert.Equals("ana@example.com", ComposerTestSetup{}.Auth(test.behaviors).Address())
}
