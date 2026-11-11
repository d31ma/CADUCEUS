// @Test(AdminShellTestSetup, {"partition":"tests"})
type InitialsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *InitialsServiceTest) TestOfTakesTheFirstLetterOfTwoWords() {
	Assert.Equals("AR", (&InitialsService{}).Of("ana ruiz lopez"))
}

func (test *InitialsServiceTest) TestOfWorksWithOneWordAndNoWords() {
	Assert.Equals("O", (&InitialsService{}).Of("Olu"))
	Assert.Equals("", (&InitialsService{}).Of("  "))
}
