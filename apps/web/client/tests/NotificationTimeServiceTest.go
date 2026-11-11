// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationTimeServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationTimeServiceTest) times() *NotificationTimeService {
	NotificationTestSetup{}.answerBrowser()
	return NotificationTestSetup{}.times()
}

func (test *NotificationTimeServiceTest) TestStampAndLaterAreUniversalTime() {
	times := test.times()
	Assert.Equals("2026-11-10T11:00:00Z", times.Stamp())
	Assert.Equals("2026-11-10T12:30:00Z", times.Later(90))
	Assert.Equals("2026-11-11T09:00:00Z", times.TomorrowMorning())
}

func (test *NotificationTimeServiceTest) TestClockTextUsesTheLocalZone() {
	Assert.Equals("10:05", test.times().ClockText("2026-11-10T10:05:00Z"))
}

func (test *NotificationTimeServiceTest) TestClockTextWithoutAZoneUsesTheDeviceZone() {
	NotificationTestSetup{}.answerBrowser()
	Assert.Equals(5, len((&NotificationTimeService{}).ClockText("2026-11-10T10:05:00Z")))
}

func (test *NotificationTimeServiceTest) TestDayTextNamesRecentDays() {
	times := test.times()
	Assert.Equals("Today", times.DayText("2026-11-10T00:10:00Z"))
	Assert.Equals("Yesterday", times.DayText("2026-11-09T23:50:00Z"))
	Assert.Equals("Tuesday 3 November", times.DayText("2026-11-03T08:00:00Z"))
}

func (test *NotificationTimeServiceTest) TestEndTextNamesTheDayWhenNotToday() {
	times := test.times()
	Assert.Equals("17:00", times.EndText("2026-11-10T17:00:00Z"))
	Assert.Equals("tomorrow 09:00", times.EndText("2026-11-11T09:00:00Z"))
	Assert.Equals("Friday 09:00", times.EndText("2026-11-13T09:00:00Z"))
}

func (test *NotificationTimeServiceTest) TestPassed() {
	times := test.times()
	Assert.IsTrue(times.Passed("2026-11-10T11:00:00Z"))
	Assert.IsTrue(!times.Passed("2026-11-10T11:00:01Z"))
	Assert.IsTrue(!times.Passed(""))
}
