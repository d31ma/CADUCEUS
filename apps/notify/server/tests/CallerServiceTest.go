import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type CallerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CallerServiceTest) TestCallerReadsTheDomainAndMember() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	caller, err := (&CallerService{}).Caller(setup.Member(member, "member-1"))
	Assert.IsTrue(err == nil, "caller refused")
	Assert.Equals(domainId, caller.DomainId)
	Assert.Equals("member-1", caller.MemberId)
}

func (test *CallerServiceTest) TestCallerRefusesABadToken() {
	NotifyTestSetup{}.Realms(NotifyTestSetup{}.Domain())
	_, err := (&CallerService{}).Caller("Bearer nonsense")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused))
}

func (test *CallerServiceTest) TestOperatorAcceptsAnInstallWideToken() {
	setup := NotifyTestSetup{}
	_, installWide := setup.Realms(setup.Domain())
	Assert.IsTrue((&CallerService{}).Operator(setup.Service(installWide)) == nil)
}

func (test *CallerServiceTest) TestOperatorRefusesADomainService() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	err := (&CallerService{}).Operator(setup.Service(member))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *CallerServiceTest) TestIsInstallWideMatchesTheStarDomain() {
	Assert.IsTrue((&CallerService{}).IsInstallWide("*"), "the star domain was not install-wide")
}

func (test *CallerServiceTest) TestIsInstallWideRefusesANamedDomain() {
	Assert.IsTrue(!(&CallerService{}).IsInstallWide("example.com"), "a named domain was install-wide")
}
