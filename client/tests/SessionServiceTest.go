// SessionServiceTest covers reading the session that a tab already holds.
// @Test(ShellTestSetup, {"partition":"tests"})
type SessionServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionServiceTest) TestRestoreReadsTheLegacyKeys() {
	session := &SessionService{}
	session.Restore(StoredValues{Session: map[string]string{
		"caduceus_token": "t", "caduceus_email": "jane.doe@example.com", "caduceus_role": "admin", "caduceus_domains": `["example.com"]`,
	}})
	Assert.IsTrue(session.SignedIn)
	Assert.IsTrue(session.Restored)
	Assert.Equals("Jane Doe", session.DisplayName)
	Assert.Equals([]string{"example.com"}, session.Domains)
}

func (test *SessionServiceTest) TestAnEmptyTabIsSignedOut() {
	session := &SessionService{}
	session.Restore(StoredValues{Session: map[string]string{"caduceus_domains": "not json"}})
	Assert.Equals(false, session.SignedIn)
	Assert.Equals([]string{}, session.Domains)
}

func (test *SessionServiceTest) TestAnIncompleteTokenSetIsRefused() {
	session := &SessionService{}
	session.HandleLogin(TokenSet{AccessToken: "t"})
	Assert.Equals(false, session.SignedIn)
}
