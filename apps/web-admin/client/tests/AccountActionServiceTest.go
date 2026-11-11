// @Test(PeopleDirectoryTestSetup, {"partition":"tests"})
type AccountActionServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *AccountActionServiceTest) asked(dialog string, personId string) *AccountActionService {
	session := &AdminSessionService{}
	test.construct(session)
	account := &AccountActionService{session: session, problems: &AdminProblemService{}, rules: &UsernameRuleService{}}
	test.construct(account)
	account.Ask(dialog, personId, "Dan Example", "dan")
	return account
}

func (test *AccountActionServiceTest) TestAskOpensTheDialogForThePerson() {
	account := test.asked("reset", "dan-id")
	Assert.Equals("reset", account.Dialog)
	Assert.Equals("Dan", account.FirstName)
	Assert.Equals("dan", account.Username)
}

func (test *AccountActionServiceTest) TestResetWithSetsAFirstPassword() {
	account := test.asked("reset", "dan-id")
	account.ResetWith("a long first password")
	Assert.Equals("done", account.Dialog)
	Assert.Equals("Password reset", account.DoneTitle)
	Assert.Equals(false, account.Busy)
}

func (test *AccountActionServiceTest) TestResetWithChecksTheLength() {
	account := test.asked("reset", "dan-id")
	account.ResetWith("short")
	Assert.Equals("reset", account.Dialog)
	Assert.Equals("Use at least 12 characters.", account.PasswordProblem)
}

func (test *AccountActionServiceTest) TestResetWithShowsThePolicyRefusal() {
	account := test.asked("reset", "dan-id")
	account.ResetWith("weakpassword1")
	Assert.Equals("reset", account.Dialog)
	Assert.Equals("Mesail did not accept this password. Use at least 12 characters, and not the username or the address.", account.PasswordProblem)
}

func (test *AccountActionServiceTest) TestAPersonInAnotherDomainIsForbidden() {
	account := test.asked("reset", "eve-id")
	account.ResetWith("a long first password")
	Assert.Equals("forbidden", account.Dialog)
}

func (test *AccountActionServiceTest) TestOfflineOpensTheFailedDialog() {
	account := test.asked("totp", "offline-id")
	account.RemoveTwoStepCode()
	Assert.Equals("failed", account.Dialog)
}

func (test *AccountActionServiceTest) TestRemoveTotpRemovesTheCode() {
	account := test.asked("totp", "dan-id")
	account.RemoveTwoStepCode()
	Assert.Equals("done", account.Dialog)
	Assert.Equals("2-step code removed", account.DoneTitle)
}

func (test *AccountActionServiceTest) TestTurnOffTurnsTheAccountOff() {
	account := test.asked("disable", "dan-id")
	account.TurnOff()
	Assert.Equals("done", account.Dialog)
	Assert.Equals("Account turned off", account.DoneTitle)
}

func (test *AccountActionServiceTest) TestTurnOnNeedsNoDialog() {
	account := test.asked("", "dan-id")
	account.TurnOn("dan-id", "Dan Example")
	Assert.Equals("", account.Dialog)
	Assert.Equals("Dan Example can sign in again.", account.Notice)
}

func (test *AccountActionServiceTest) TestRenameWithChangesTheName() {
	account := test.asked("rename", "dan-id")
	account.RenameWith(" Daniel Example ")
	Assert.Equals("done", account.Dialog)
	Assert.Equals("Everyone now sees Daniel Example, in every domain of Dan.", account.DoneText)
}

func (test *AccountActionServiceTest) TestRenameWithAnEmptyNameAsksForOne() {
	account := test.asked("rename", "dan-id")
	account.RenameWith("  ")
	Assert.Equals("rename", account.Dialog)
	Assert.Equals("Type the name people see, like Dan Example.", account.NameProblem)
}

func (test *AccountActionServiceTest) TestDeleteWithNeedsTheTypedUsername() {
	account := test.asked("delete", "dan-id")
	account.DeleteWith("Dan")
	Assert.Equals("delete", account.Dialog)
	Assert.Equals("Type dan to confirm.", account.ConfirmProblem)
	account.DeleteWith(" dan ")
	Assert.Equals("done", account.Dialog)
	Assert.Equals("Person deleted", account.DoneTitle)
	Assert.Equals("Dan can no longer sign in. Mesail now removes Dan from every domain, in the background. The username dan is never given to anyone else.", account.DoneText)
}

func (test *AccountActionServiceTest) TestCloseHidesTheDialog() {
	account := test.asked("rename", "dan-id")
	account.Close()
	Assert.Equals("", account.Dialog)
}
