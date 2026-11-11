// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationPushServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationPushServiceTest) build(domainId string) *NotificationPushService {
	return NotificationTestSetup{}.push(test.behaviors, NotificationTestSetup{}.access(test.behaviors, domainId))
}

func (test *NotificationPushServiceTest) loaded(permission any) *NotificationPushService {
	service := test.build("example.com")
	BrowserFakes{}.Answer("notifications", "permission", permission)
	service.LoadSettings()
	return service
}

func (test *NotificationPushServiceTest) unread() []Notification {
	return []Notification{{NotificationId: "n1", Kind: "mention", CreatedAt: "2026-11-10T10:05:00Z"}}
}

func (test *NotificationPushServiceTest) TestStartWaitsForTheSettings() {
	service := test.build("example.com")
	service.Start()
	Assert.Equals("loading", service.SettingsStatus)
	Assert.Equals(6, len(service.SummaryChoices))
	Assert.IsTrue(service.SummaryChoices[0].Selected)
}

func (test *NotificationPushServiceTest) TestLoadSettingsReadsPermissionAndDelay() {
	service := test.loaded("granted")
	Assert.Equals("on", service.Panel)
	Assert.Equals("ready", service.SettingsStatus)
	Assert.Equals(30, service.SummaryMinutes)
	Assert.Equals("summary-30", service.SummaryChoices[2].Id)
	Assert.IsTrue(service.SummaryChoices[2].Selected)
}

func (test *NotificationPushServiceTest) TestLoadSettingsFailureShowsTheErrorState() {
	service := test.build("broken.example")
	service.LoadSettings()
	Assert.Equals("error", service.SettingsStatus)
}

func (test *NotificationPushServiceTest) TestPermissionNotAskedYet() {
	Assert.Equals("notAsked", test.loaded("default").Panel)
}

func (test *NotificationPushServiceTest) TestPermissionBlockedByTheBrowser() {
	Assert.Equals("blocked", test.loaded("denied").Panel)
}

func (test *NotificationPushServiceTest) TestBrowserWithoutNotifications() {
	Assert.Equals("unsupported", test.loaded(5).Panel)
}

func (test *NotificationPushServiceTest) TestTurnOnAsksTheBrowserThenListens() {
	service := test.loaded("default")
	service.TurnOn()
	Assert.Equals("", service.Problem)
	Assert.Equals("notAsked", service.Panel)
	service.PermissionChanged(NotificationPermissionChanged{Permission: "granted"})
	Assert.Equals("on", service.Panel)
}

func (test *NotificationPushServiceTest) TestCallFailedShowsAProblem() {
	service := test.loaded("default")
	service.CallFailed(BrowserCallFailed{})
	Assert.Equals("notAllowed", service.Problem)
}

func (test *NotificationPushServiceTest) TestTurnOffAndOnAgainOnThisDevice() {
	service := test.loaded("granted")
	service.TurnOff()
	Assert.Equals("off", service.Panel)
	Assert.IsTrue(service.DeviceOff)
	service.TurnOn()
	Assert.Equals("on", service.Panel)
}

func (test *NotificationPushServiceTest) TestCheckAgainReadsThePermission() {
	service := test.loaded("denied")
	BrowserFakes{}.Answer("notifications", "permission", "granted")
	service.CheckAgain()
	Assert.Equals("on", service.Panel)
}

func (test *NotificationPushServiceTest) TestSendTestShowsANotification() {
	service := test.loaded("granted")
	service.SendTest()
	Assert.IsTrue(service.TestSent)
	Assert.Equals("", service.Problem)
}

func (test *NotificationPushServiceTest) TestChooseSummarySavesTheDelay() {
	service := test.loaded("granted")
	service.ChooseSummary(240)
	Assert.Equals(240, service.SummaryMinutes)
	Assert.IsTrue(service.Saved)
	Assert.IsTrue(service.SummaryChoices[4].Selected)
}

func (test *NotificationPushServiceTest) TestChooseTheSameDelayAgainSendsNothing() {
	service := test.build("broken.example")
	service.SummaryMinutes = 60
	service.ChooseSummary(60)
	Assert.Equals("", service.SaveProblem)
}

func (test *NotificationPushServiceTest) TestDomainChangedClearsTheOldSettings() {
	service := test.loaded("granted")
	service.ChooseSummary(240)
	service.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	Assert.Equals(0, service.SummaryMinutes)
	Assert.IsTrue(!service.Saved)
	Assert.Equals("loading", service.SettingsStatus)
}

func (test *NotificationPushServiceTest) TestChooseSummaryKeepsTheOldDelayWhenSavingFails() {
	service := test.build("broken.example")
	service.SummaryMinutes = 30
	service.ChooseSummary(60)
	Assert.Equals(30, service.SummaryMinutes)
	Assert.Equals("error", service.SaveProblem)
	Assert.IsTrue(!service.Saved)
}

func (test *NotificationPushServiceTest) TestAnnounceShowsEachItemOnce() {
	service := test.loaded("granted")
	service.Announce(test.unread(), true)
	service.Announce(test.unread(), true)
	Assert.Equals(map[string]bool{"n1": true}, service.shown)
	Assert.Equals("", service.Problem)
}

func (test *NotificationPushServiceTest) TestAnnounceWhileVisibleShowsNothing() {
	service := test.loaded("granted")
	service.Announce(test.unread(), false)
	Assert.Equals(0, len(service.shown))
}

func (test *NotificationPushServiceTest) TestAnnounceWhenTurnedOffShowsNothing() {
	service := test.loaded("granted")
	service.TurnOff()
	service.Announce(test.unread(), true)
	Assert.Equals(0, len(service.shown))
}

func (test *NotificationPushServiceTest) TestAnnounceDuringDoNotDisturbShowsNothing() {
	service := test.loaded("granted")
	service.presence.DoNotDisturb("30m")
	service.Announce(test.unread(), true)
	Assert.Equals(0, len(service.shown))
}
