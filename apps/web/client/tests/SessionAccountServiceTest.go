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

func (test *SessionAccountServiceTest) TestRestoreMakesTheFirstAccountCurrent() {
	accounts := &SessionAccountService{}
	accounts.Restore([]Me{test.me("example.com", "ana@example.com"), test.me("example.org", "ana@example.org")})
	Assert.Equals(2, len(accounts.Accounts))
	Assert.Equals("example.com", accounts.CurrentDomain)
	Assert.IsTrue(accounts.Accounts[0].Current)
}

func (test *SessionAccountServiceTest) TestRestoreOfNothingLeavesNoAccount() {
	accounts := &SessionAccountService{}
	accounts.Restore([]Me{})
	Assert.Equals("", accounts.CurrentDomain)
}

func (test *SessionAccountServiceTest) TestDomainsWithTheSameStartGetTheirEndingLetter() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	accounts.Add(test.me("example.org", "ana@example.org"))
	accounts.Add(test.me("lab.example.net", "ana@lab.example.net"))
	Assert.Equals("Ec", accounts.Accounts[0].Initials)
	Assert.Equals("Eo", accounts.Accounts[1].Initials)
	Assert.Equals("La", accounts.Accounts[2].Initials)
}

func (test *SessionAccountServiceTest) TestAddKeepsTheMemberId() {
	accounts := &SessionAccountService{}
	accounts.Add(test.me("example.com", "ana@example.com"))
	Assert.Equals("m-1", accounts.Current().MemberId)
}
