// @Test(SearchTestSetup, {"partition":"tests"})
type SearchTimeServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchTimeServiceTest) times() *SearchTimeService {
	BrowserFakes{}.Answer("clock", "now", 1794391200000)
	BrowserFakes{}.Answer("clock", "timeZone", "Europe/Berlin")
	return &SearchTimeService{}
}

func (test *SearchTimeServiceTest) TestLabelOfNamesTodayAndYesterdayInTheLocalZone() {
	times := test.times()
	Assert.Equals("Today 08:52", times.LabelOf("2026-11-11T07:52:00.123Z"))
	Assert.Equals("Yesterday 16:20", times.LabelOf("2026-11-10T15:20:00Z"))
}

func (test *SearchTimeServiceTest) TestLabelOfShowsTheDateForOlderMessages() {
	times := test.times()
	Assert.Equals("3 Nov, 10:00", times.LabelOf("2026-11-03T09:00:00Z"))
	Assert.Equals("28 Oct 2025, 10:00", times.LabelOf("2025-10-28T09:00:00Z"))
}

func (test *SearchTimeServiceTest) TestLabelOfWithAnUnknownZoneUsesUtc() {
	BrowserFakes{}.Answer("clock", "now", 1794391200000)
	BrowserFakes{}.Answer("clock", "timeZone", "Nowhere/City")
	Assert.Equals("Today 07:52", (&SearchTimeService{}).LabelOf("2026-11-11T07:52:00Z"))
}

func (test *SearchTimeServiceTest) TestLabelOfKeepsTextItCannotRead() {
	Assert.Equals("yesterday", test.times().LabelOf("yesterday"))
}
