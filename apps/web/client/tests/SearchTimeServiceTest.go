// @Test(SearchTestSetup, {"partition":"tests"})
type SearchTimeServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchTimeServiceTest) TestLabelOfShowsTheDayAndTimeInUtc() {
	Assert.Equals("10 Nov 2026, 08:52 UTC", (&SearchTimeService{}).LabelOf("2026-11-10T09:52:00.123+01:00"))
}

func (test *SearchTimeServiceTest) TestLabelOfKeepsTextItCannotRead() {
	Assert.Equals("yesterday", (&SearchTimeService{}).LabelOf("yesterday"))
}
