// @Test(AdminShellTestSetup, {"partition":"tests"})
type UsernameRuleServiceTest struct {
	behaviors *BehaviorContext
}

func (test *UsernameRuleServiceTest) TestCleanedTrimsAndLowers() {
	Assert.Equals("dan", (&UsernameRuleService{}).Cleaned("  Dan "))
}

func (test *UsernameRuleServiceTest) TestAGoodUsernameHasNoProblem() {
	Assert.Equals("", (&UsernameRuleService{}).Problem("Dan.Example-1"))
}

func (test *UsernameRuleServiceTest) TestAnEmptyUsernameAsksForOne() {
	Assert.Equals("Type a username, like dan.", (&UsernameRuleService{}).Problem("  "))
}

func (test *UsernameRuleServiceTest) TestASpaceBreaksTheRules() {
	Assert.Equals("Use 1 to 64 letters, digits, dots, dashes or underscores. Start and end with a letter or digit.", (&UsernameRuleService{}).Problem("dan smith"))
}

func (test *UsernameRuleServiceTest) TestPunctuationAtTheEndBreaksTheRules() {
	Assert.Equals("Use 1 to 64 letters, digits, dots, dashes or underscores. Start and end with a letter or digit.", (&UsernameRuleService{}).Problem("dan."))
}

func (test *UsernameRuleServiceTest) TestTwoDotsBreakTheRules() {
	Assert.Equals("Use 1 to 64 letters, digits, dots, dashes or underscores. Start and end with a letter or digit.", (&UsernameRuleService{}).Problem("dan..x"))
}

func (test *UsernameRuleServiceTest) TestAnAddressBreaksTheRules() {
	Assert.Equals("Use 1 to 64 letters, digits, dots, dashes or underscores. Start and end with a letter or digit.", (&UsernameRuleService{}).Problem("dan@example.com"))
}

func (test *UsernameRuleServiceTest) TestARoleNameIsKeptForSystemMail() {
	Assert.Equals("postmaster is kept for system mail. Choose another username, or add postmaster as an alias.", (&UsernameRuleService{}).Problem("Postmaster"))
}

func (test *UsernameRuleServiceTest) TestAShortPasswordIsRefused() {
	Assert.Equals("Use at least 12 characters.", (&UsernameRuleService{}).PasswordProblem("short"))
	Assert.Equals("", (&UsernameRuleService{}).PasswordProblem("long enough pass"))
}
