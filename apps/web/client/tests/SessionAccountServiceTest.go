// @Test(SessionTestSetup, {"partition":"tests"})
type SessionAccountServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionAccountServiceTest) me(domainId string, address string) Me {
	return Me{MemberId: "m-1", DomainId: domainId, DisplayName: "Ana Ruiz", PrimaryAddress: address}
}

func (test *SessionAccountServiceTest) TestAddMakesTheAccountCurrent() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	Assert.Equals("example.com", accounts.CurrentDomain)
	Assert.Equals("Ex", accounts.Current().Initials)
	Assert.IsTrue(accounts.Current().Current)
}

func (test *SessionAccountServiceTest) TestAddKeepsOneAccountPerDomain() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	accounts.Add(test.me("example.com", "ana.ruiz@example.com"))
	Assert.Equals(1, len(accounts.Accounts))
	Assert.Equals("ana.ruiz@example.com", accounts.Current().Address)
}

func (test *SessionAccountServiceTest) TestSwitchMovesTheCurrentMark() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	accounts.Add(test.me("example.org", "ana@example.org"))
	accounts.Switch("example.com")
	Assert.IsTrue(accounts.Accounts[0].Current)
	Assert.IsTrue(!accounts.Accounts[1].Current)
}

func (test *SessionAccountServiceTest) TestRemoveSwitchesToTheFirstOtherAccount() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	accounts.Add(test.me("example.org", "ana@example.org"))
	accounts.Remove("example.org")
	Assert.Equals("example.com", accounts.CurrentDomain)
}

func (test *SessionAccountServiceTest) TestRemoveOfTheLastAccountLeavesNoCurrentDomain() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	accounts.Remove("example.com")
	Assert.Equals("", accounts.CurrentDomain)
	Assert.Equals("", accounts.Current().DomainId)
}

func (test *SessionAccountServiceTest) TestLoadAsksWhoIsSignedIn() {
	client := NewIdentityClientWith(test.behaviors)
	me, failure := (&SessionAccountService{identity: client}).Load("access-1")
	Assert.Equals(nil, failure)
	Assert.Equals("ana@example.com", me.PrimaryAddress)
}

func (test *SessionAccountServiceTest) TestLoadReportsARefusedToken() {
	client := NewIdentityClientWith(test.behaviors)
	_, failure := (&SessionAccountService{identity: client}).Load("stale")
	Assert.IsTrue(failure != nil)
}
