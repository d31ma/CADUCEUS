// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationPresenceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationPresenceServiceTest) build(domainId string) *NotificationPresenceService {
	return NotificationTestSetup{}.presence(test.behaviors, NotificationTestSetup{}.access(test.behaviors, domainId))
}

func (test *NotificationPresenceServiceTest) watching() *NotificationPresenceService {
	service := test.build("example.com")
	service.Load(PresenceWatchRequested{MemberIds: []string{"m-2", "m-3", "m-x"}})
	return service
}

func (test *NotificationPresenceServiceTest) TestStartIsActive() {
	service := test.build("example.com")
	service.MyState = ""
	service.Start()
	Assert.Equals("active", service.MyState)
}

func (test *NotificationPresenceServiceTest) TestShownFollowsThePage() {
	service := test.build("example.com")
	service.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("idle", service.MyState)
	service.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("active", service.MyState)
}

func (test *NotificationPresenceServiceTest) TestShownFailureIsKept() {
	service := test.build("broken.example")
	service.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("error", service.Problem)
}

func (test *NotificationPresenceServiceTest) TestDoNotDisturbForThirtyMinutes() {
	service := test.build("example.com")
	service.ToggleMenu()
	service.DoNotDisturb("30m")
	Assert.Equals("dnd", service.MyState)
	Assert.Equals("2026-11-10T11:30:00Z", service.DoNotDisturbUntil)
	Assert.IsTrue(!service.MenuOpen)
	Assert.IsTrue(service.Quiet())
	Assert.Equals("Do not disturb until 11:30", service.StatusText())
	Assert.IsTrue(service.Choices[0].Selected)
}

func (test *NotificationPresenceServiceTest) TestDoNotDisturbUntilTomorrowMorning() {
	service := test.build("example.com")
	service.DoNotDisturb("tomorrow")
	Assert.Equals("2026-11-11T09:00:00Z", service.DoNotDisturbUntil)
	Assert.Equals("Do not disturb until tomorrow 09:00", service.StatusText())
}

func (test *NotificationPresenceServiceTest) TestDoNotDisturbStaysWhileHidden() {
	service := test.build("example.com")
	service.DoNotDisturb("1h")
	service.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("dnd", service.MyState)
}

func (test *NotificationPresenceServiceTest) TestDoNotDisturbFailureKeepsTheMenuOpen() {
	service := test.build("broken.example")
	service.ToggleMenu()
	service.DoNotDisturb("2h")
	Assert.Equals("active", service.MyState)
	Assert.Equals("error", service.Problem)
	Assert.IsTrue(service.MenuOpen)
}

func (test *NotificationPresenceServiceTest) TestSetActiveClearsDoNotDisturb() {
	service := test.build("example.com")
	service.DoNotDisturb("2h")
	service.SetActive()
	Assert.Equals("active", service.MyState)
	Assert.Equals("", service.DoNotDisturbUntil)
	Assert.IsTrue(!service.Quiet())
}

func (test *NotificationPresenceServiceTest) TestDoNotDisturbEndsWhenItsTimeHasPassed() {
	service := test.build("example.com")
	service.DoNotDisturb("30m")
	BrowserFakes{}.Answer("clock", "now", 1794310260000)
	Assert.IsTrue(!service.Quiet())
	service.ToggleMenu()
	Assert.Equals("active", service.MyState)
	Assert.Equals("", service.DoNotDisturbUntil)
}

func (test *NotificationPresenceServiceTest) TestToggleMenuShowsTheEndTimes() {
	service := test.build("example.com")
	service.ToggleMenu()
	Assert.IsTrue(service.MenuOpen)
	Assert.Equals("Until 11:30", service.Choices[0].EndText)
	Assert.Equals("Until 12:00", service.Choices[1].EndText)
	Assert.Equals("Until 13:00", service.Choices[2].EndText)
	Assert.Equals("Until tomorrow 09:00", service.Choices[3].EndText)
	service.CloseMenu()
	Assert.IsTrue(!service.MenuOpen)
}

func (test *NotificationPresenceServiceTest) TestWatchRequestLoadsTheStatesOfMembers() {
	service := test.watching()
	Assert.Equals("active", service.stateOf("m-2"))
	Assert.Equals("dnd", service.stateOf("m-3"))
	Assert.Equals("offline", service.stateOf("m-x"))
	Assert.Equals(", do not disturb", service.labelOf("m-3"))
	Assert.Equals(", offline", service.labelOf("m-unknown"))
}

func (test *NotificationPresenceServiceTest) TestWatchRequestsAddUp() {
	service := test.watching()
	service.Load(PresenceWatchRequested{MemberIds: []string{"m-4"}})
	Assert.Equals("active", service.stateOf("m-2"))
	Assert.Equals("idle", service.stateOf("m-4"))
}

func (test *NotificationPresenceServiceTest) TestLoadForgetsTheOtherDomain() {
	service := test.watching()
	service.access.accounts.Add(Me{MemberId: "m-1", DomainId: "broken.example", DisplayName: "Ana Ruiz"})
	service.Load(PresenceWatchRequested{MemberIds: []string{"m-4"}})
	Assert.Equals("offline", service.stateOf("m-2"))
	Assert.Equals("error", service.LoadProblem)
}

func (test *NotificationPresenceServiceTest) TestOpeningTheMenuSendsActive() {
	service := test.build("example.com")
	service.MyState = "idle"
	service.ToggleMenu()
	Assert.Equals("active", service.MyState)
	Assert.Equals("", service.Problem)
}

func (test *NotificationPresenceServiceTest) TestDomainChangedStartsActiveInTheNewDomain() {
	service := test.watching()
	service.DoNotDisturb("1h")
	service.access.accounts.Add(Me{MemberId: "m-1", DomainId: "broken.example", DisplayName: "Ana Ruiz"})
	service.DomainChanged(SessionDomainChanged{DomainId: "broken.example"})
	Assert.Equals("active", service.MyState)
	Assert.Equals("", service.DoNotDisturbUntil)
	Assert.Equals("offline", service.stateOf("m-2"))
}

func (test *NotificationPresenceServiceTest) TestNameInitialsAndButtonLabel() {
	service := test.build("example.com")
	Assert.Equals("Ana Ruiz", service.Name())
	Assert.Equals("AR", service.Initials())
	Assert.Equals("Ana Ruiz, Active. Open status menu", service.ButtonLabel())
}

func (test *NotificationPresenceServiceTest) TestNameFallsBackToTheAddress() {
	service := test.build("example.com")
	service.access.accounts.Accounts[0].DisplayName = ""
	Assert.Equals("ana@example.com", service.Name())
	Assert.Equals("A", service.Initials())
}
