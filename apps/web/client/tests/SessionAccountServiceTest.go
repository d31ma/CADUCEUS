// @Test(SessionTestSetup, {"partition":"tests"})
type SessionAccountServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionAccountServiceTest) session(domainId string, choices ...DomainChoice) Session {
	me := Me{MemberId: "m-1", DomainId: domainId, Username: "ana", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@" + domainId}
	return Session{Me: me, Domains: choices, SessionKey: "key-1"}
}

func (test *SessionAccountServiceTest) applied() *SessionAccountService {
	accounts := &SessionAccountService{}
	accounts.Apply(test.session("example.org", DomainChoice{DomainId: "example.org"}, DomainChoice{DomainId: "example.com", Admin: true, Default: true}, DomainChoice{DomainId: "lab.example.net"}))
	return accounts
}

func (test *SessionAccountServiceTest) TestApplyMakesTheActiveDomainCurrent() {
	accounts := test.applied()
	Assert.Equals("example.org", accounts.CurrentDomain)
	Assert.Equals("example.org", accounts.Current().DomainId)
	Assert.IsTrue(accounts.Current().Current)
}

func (test *SessionAccountServiceTest) TestApplyPutsTheDefaultDomainFirst() {
	accounts := test.applied()
	Assert.Equals("example.com", accounts.Accounts[0].DomainId)
	Assert.Equals("example.org", accounts.Accounts[1].DomainId)
	Assert.Equals("lab.example.net", accounts.Accounts[2].DomainId)
}

func (test *SessionAccountServiceTest) TestApplyKeepsTheAdminAndDefaultTags() {
	accounts := test.applied()
	Assert.IsTrue(accounts.Accounts[0].Admin)
	Assert.IsTrue(accounts.Accounts[0].Default)
	Assert.Equals(false, accounts.Accounts[1].Admin)
}

func (test *SessionAccountServiceTest) TestApplyBuildsTheAddressFromTheUsername() {
	accounts := test.applied()
	Assert.Equals("ana@example.com", accounts.Accounts[0].Address)
	Assert.Equals("ana", accounts.Username)
	Assert.Equals("Ana Ruiz", accounts.DisplayName)
}

func (test *SessionAccountServiceTest) TestApplyKeepsTheMemberId() {
	Assert.Equals("m-1", test.applied().Current().MemberId)
}

func (test *SessionAccountServiceTest) TestDomainsWithTheSameStartGetTheirEndingLetter() {
	accounts := test.applied()
	Assert.Equals("Ec", accounts.Accounts[0].Initials)
	Assert.Equals("Eo", accounts.Accounts[1].Initials)
	Assert.Equals("La", accounts.Accounts[2].Initials)
}

func (test *SessionAccountServiceTest) TestFirstApplyMarksNothingNew() {
	accounts := test.applied()
	Assert.Equals(false, accounts.Accounts[0].New)
	Assert.Equals(false, accounts.Accounts[2].New)
}

func (test *SessionAccountServiceTest) TestAnAddedDomainIsNewUntilSeen() {
	accounts := test.applied()
	accounts.Apply(test.session("example.org", DomainChoice{DomainId: "example.org"}, DomainChoice{DomainId: "example.com", Default: true}, DomainChoice{DomainId: "partner.test"}))
	Assert.IsTrue(accounts.Accounts[2].New)
	accounts.MarkSeen()
	Assert.Equals(false, accounts.Accounts[2].New)
}

func (test *SessionAccountServiceTest) TestRemoveDropsTheDomain() {
	accounts := test.applied()
	accounts.Remove("lab.example.net")
	Assert.Equals(2, len(accounts.Accounts))
}

func (test *SessionAccountServiceTest) TestMarkDefaultMovesTheDefaultFirst() {
	accounts := test.applied()
	accounts.MarkDefault("lab.example.net")
	Assert.Equals("lab.example.net", accounts.Accounts[0].DomainId)
	Assert.IsTrue(accounts.Accounts[0].Default)
	Assert.Equals("lab.example.net", accounts.Default().DomainId)
}

func (test *SessionAccountServiceTest) TestOtherDomainIsTheFirstThatIsNotDefault() {
	Assert.Equals("example.org", test.applied().OtherDomain())
}

func (test *SessionAccountServiceTest) TestDomainListJoinsTheDomains() {
	Assert.Equals("example.com,example.org,lab.example.net", test.applied().DomainList())
}

func (test *SessionAccountServiceTest) TestSingleIsTrueForOneDomain() {
	accounts := &SessionAccountService{}
	accounts.Apply(test.session("example.com", DomainChoice{DomainId: "example.com", Default: true}))
	Assert.IsTrue(accounts.Single())
	Assert.Equals("", accounts.OtherDomain())
}

func (test *SessionAccountServiceTest) TestClearForgetsEverything() {
	accounts := test.applied()
	accounts.Clear()
	Assert.Equals(0, len(accounts.Accounts))
	Assert.Equals("", accounts.CurrentDomain)
	Assert.Equals("", accounts.Username)
}
