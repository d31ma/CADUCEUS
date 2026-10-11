// CalendarServiceTest covers the calendar's answers to the shell's events
// before anything is loaded.
// @Test(ShellTestSetup, {"partition":"tests"})
type CalendarServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CalendarServiceTest) calendar() *CalendarService {
	BrowserFakes{}.Answer("clock", "timeZone", "UTC")
	calendar := &CalendarService{}
	calendar.Start()
	return calendar
}

func (test *CalendarServiceTest) TestAPhoneSizedScreenShowsTheDayForAWideView() {
	calendar := test.calendar()
	Assert.Equals("month", calendar.ViewMode)
	calendar.HearViewport(ViewportMeasured{Narrow: true})
	Assert.Equals("month", calendar.ViewMode)
	calendar.ViewMode = "week"
	calendar.HearViewport(ViewportMeasured{Narrow: true})
	Assert.Equals("day", calendar.ViewMode)
	calendar.HearViewport(ViewportMeasured{Narrow: false})
	Assert.Equals("day", calendar.ViewMode)
}

func (test *CalendarServiceTest) TestTheQuickEventClosesOnTheBackdropAndEscape() {
	calendar := test.calendar()
	calendar.OpenCell("2026-10-12", "calendar-cell")
	Assert.IsTrue(calendar.QuickCreateOpen)
	calendar.HearBackdrop(BackdropClicked{Dialog: "calendar-event-dialog"})
	Assert.IsTrue(calendar.QuickCreateOpen)
	calendar.HearBackdrop(BackdropClicked{Dialog: "calendar-quick-event"})
	Assert.Equals(false, calendar.QuickCreateOpen)
	calendar.OpenCell("2026-10-12", "calendar-cell")
	calendar.HearEscape(EscapePressed{})
	Assert.Equals(false, calendar.QuickCreateOpen)
}

func (test *CalendarServiceTest) TestAChipClickOpensNoQuickEvent() {
	calendar := test.calendar()
	calendar.OpenCell("2026-10-12", "calendar-event-chip")
	Assert.Equals(false, calendar.QuickCreateOpen)
}

func (test *CalendarServiceTest) TestTheNewEventDialogOpensAndCloses() {
	calendar := test.calendar()
	calendar.OpenCreateModal()
	Assert.IsTrue(calendar.ModalOpen)
	calendar.HearBackdrop(BackdropClicked{Dialog: "calendar-event-dialog"})
	Assert.Equals(false, calendar.ModalOpen)
}
