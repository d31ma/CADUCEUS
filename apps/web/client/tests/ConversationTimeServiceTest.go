// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationTimeServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationTimeServiceTest) now() float64 {
	return 1794340800000
}

func (test *ConversationTimeServiceTest) universal() string {
	return "UTC"
}

func (test *ConversationTimeServiceTest) newYork() string {
	return "America/New_York"
}

func (test *ConversationTimeServiceTest) TestTimeShowsHoursAndMinutes() {
	Assert.Equals("14:05", (&ConversationTimeService{}).Time("2026-11-10T14:05:00Z", test.universal()))
}

func (test *ConversationTimeServiceTest) TestTimeUsesUniversalTime() {
	Assert.Equals("14:05", (&ConversationTimeService{}).Time("2026-11-10T15:05:00+01:00", test.universal()))
}

func (test *ConversationTimeServiceTest) TestTimeUsesTheLocalTimeZone() {
	Assert.Equals("09:05", (&ConversationTimeService{}).Time("2026-11-10T14:05:00Z", test.newYork()))
}

func (test *ConversationTimeServiceTest) TestTimeFollowsSummerTimeInTheLocalTimeZone() {
	Assert.Equals("10:05", (&ConversationTimeService{}).Time("2026-07-10T14:05:00Z", test.newYork()))
}

func (test *ConversationTimeServiceTest) TestTimeWithAnUnknownTimeZoneUsesUniversalTime() {
	Assert.Equals("14:05", (&ConversationTimeService{}).Time("2026-11-10T14:05:00Z", "Mars/Olympus_Mons"))
}

func (test *ConversationTimeServiceTest) TestTimeWithNoTimeZoneUsesUniversalTime() {
	Assert.Equals("14:05", (&ConversationTimeService{}).Time("2026-11-10T14:05:00Z", ""))
}

func (test *ConversationTimeServiceTest) TestTimeOfAnInvalidDateIsEmpty() {
	Assert.Equals("", (&ConversationTimeService{}).Time("yesterday", test.newYork()))
}

func (test *ConversationTimeServiceTest) TestDayOfTheSameDateIsToday() {
	Assert.Equals("Today", (&ConversationTimeService{}).Day("2026-11-10T00:01:00Z", test.now(), test.universal()))
}

func (test *ConversationTimeServiceTest) TestDayBeforeIsYesterday() {
	Assert.Equals("Yesterday", (&ConversationTimeService{}).Day("2026-11-09T23:59:00Z", test.now(), test.universal()))
}

func (test *ConversationTimeServiceTest) TestDayBeforeLocalMidnightIsYesterday() {
	Assert.Equals("Yesterday", (&ConversationTimeService{}).Day("2026-11-10T03:00:00Z", test.now(), test.newYork()))
}

func (test *ConversationTimeServiceTest) TestDayAfterUniversalMidnightIsStillTodayInTheLocalTimeZone() {
	Assert.Equals("Today", (&ConversationTimeService{}).Day("2026-11-11T01:00:00Z", test.now(), test.newYork()))
}

func (test *ConversationTimeServiceTest) TestOlderDayUsesTheLocalDate() {
	Assert.Equals("Sunday 1 November", (&ConversationTimeService{}).Day("2026-11-02T03:00:00Z", test.now(), test.newYork()))
}

func (test *ConversationTimeServiceTest) TestDayWithAnUnknownTimeZoneUsesUniversalTime() {
	Assert.Equals("Today", (&ConversationTimeService{}).Day("2026-11-10T03:00:00Z", test.now(), "Mars/Olympus_Mons"))
}

func (test *ConversationTimeServiceTest) TestOlderDayShowsWeekdayDateAndMonth() {
	Assert.Equals("Monday 2 November", (&ConversationTimeService{}).Day("2026-11-02T10:00:00Z", test.now(), test.universal()))
}

func (test *ConversationTimeServiceTest) TestDayOfAnotherYearShowsTheYear() {
	Assert.Equals("Wednesday 31 December 2025", (&ConversationTimeService{}).Day("2025-12-31T10:00:00Z", test.now(), test.universal()))
}

func (test *ConversationTimeServiceTest) TestDayOfAnInvalidDateIsEmpty() {
	Assert.Equals("", (&ConversationTimeService{}).Day("not a date", test.now(), test.newYork()))
}
