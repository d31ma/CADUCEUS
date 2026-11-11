// @Test(AdminShellTestSetup, {"partition":"tests"})
type AdminLoginServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AdminLoginServiceTest) TestCleanedTrimsAndLowers() {
	Assert.Equals("ana@example.com", (&AdminLoginService{}).Cleaned("  Ana@Example.COM "))
}

func (test *AdminLoginServiceTest) TestMissingFindsAnEmptyLogin() {
	Assert.Equals(true, (&AdminLoginService{}).Missing(" "))
	Assert.Equals(true, (&AdminLoginService{}).Missing("ana@"))
	Assert.Equals(false, (&AdminLoginService{}).Missing("ana"))
}

func (test *AdminLoginServiceTest) TestUsernameOfCutsAtTheLastAt() {
	Assert.Equals("ana", (&AdminLoginService{}).UsernameOf("Ana@example.com"))
	Assert.Equals("ana", (&AdminLoginService{}).UsernameOf("ana"))
}

func (test *AdminLoginServiceTest) TestDomainOfReadsThePartAfterTheAt() {
	Assert.Equals("example.com", (&AdminLoginService{}).DomainOf("ana@Example.com"))
	Assert.Equals("", (&AdminLoginService{}).DomainOf("ana"))
}

func (test *AdminLoginServiceTest) TestHintFollowsTheText() {
	Assert.Equals("Your username, like ana, or your full address.", (&AdminLoginService{}).Hint(""))
	Assert.Equals("Opens your default domain, or the whole install for an Operator.", (&AdminLoginService{}).Hint("ana"))
	Assert.Equals("Opens example.org.", (&AdminLoginService{}).Hint("ana@example.org"))
}

func (test *AdminLoginServiceTest) TestForbiddenDependsOnTheDomainPart() {
	Assert.Equals("notMember", (&AdminLoginService{}).Forbidden("ana@partner.test"))
	Assert.Equals("noDomain", (&AdminLoginService{}).Forbidden("ana"))
}
