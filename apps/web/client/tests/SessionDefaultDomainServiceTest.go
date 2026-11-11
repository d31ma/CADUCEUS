// @Test(SessionTestSetup, {"partition":"tests"})
type SessionDefaultDomainServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionDefaultDomainServiceTest) service() *SessionDefaultDomainService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	accounts := &SessionAccountService{}
	session := &SessionService{tokens: &SessionTokenService{identity: NewIdentityClientWith(test.behaviors)}, accounts: accounts, problems: &SessionProblemService{}}
	session.SignInWith("ana", "right", "", false)
	session.SignIn(SessionSignInRequested{})
	return &SessionDefaultDomainService{directory: NewDirectoryClientWith(test.behaviors), session: session, accounts: accounts, problems: &SessionProblemService{}}
}

func (test *SessionDefaultDomainServiceTest) TestSaveWithMarksTheNewDefault() {
	service := test.service()
	service.SaveWith("example.org")
	Assert.Equals("", service.Problem)
	Assert.Equals("example.org", service.Saved)
	Assert.Equals("example.org", service.accounts.Default().DomainId)
	Assert.Equals("example.org", service.accounts.Accounts[0].DomainId)
}

func (test *SessionDefaultDomainServiceTest) TestSaveWithARemovedDomainSaysSoAndDropsIt() {
	service := test.service()
	service.accounts.Accounts = append(service.accounts.Accounts, SessionAccount{DomainId: "gone.example"})
	service.SaveWith("gone.example")
	Assert.Equals("notMember", service.Problem)
	Assert.Equals("gone.example", service.ProblemDomain)
	Assert.Equals("", service.Saved)
	Assert.Equals(2, len(service.accounts.Accounts))
	Assert.Equals("example.com", service.accounts.Default().DomainId)
}

func (test *SessionDefaultDomainServiceTest) TestFirstFailureKeepsTheTokenFailure() {
	service := test.service()
	tokenFailure := &BehaviorError{Status: 401}
	Assert.Equals(error(tokenFailure), service.firstFailure(tokenFailure, nil))
	Assert.Equals(nil, service.firstFailure(nil, nil))
}
