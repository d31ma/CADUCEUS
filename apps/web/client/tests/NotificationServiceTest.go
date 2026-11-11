// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationServiceTest) build(domainId string) *NotificationService {
	return NotificationTestSetup{}.list(test.behaviors, NotificationTestSetup{}.access(test.behaviors, domainId))
}

func (test *NotificationServiceTest) loaded() *NotificationService {
	service := test.build("example.com")
	service.Start()
	service.Reload()
	return service
}

func (test *NotificationServiceTest) TestStartShowsLoadingUntilTheLoad() {
	service := test.build("example.com")
	service.Start()
	Assert.Equals("loading", service.Status)
	Assert.Equals("all", service.Tab)
	Assert.IsTrue(service.Empty)
}

func (test *NotificationServiceTest) TestLoadGroupsRowsByDay() {
	service := test.loaded()
	Assert.Equals("ready", service.Status)
	Assert.Equals(2, len(service.Days))
	Assert.Equals("Today", service.Days[0].Label)
	Assert.Equals(3, len(service.Days[0].Rows))
	Assert.Equals("Yesterday", service.Days[1].Label)
	Assert.Equals("You were mentioned", service.Days[0].Rows[0].Title)
	Assert.Equals("10:05", service.Days[0].Rows[0].Time)
	Assert.IsTrue(service.HasMore)
}

func (test *NotificationServiceTest) TestLoadCountsUnreadForBadges() {
	service := test.loaded()
	Assert.Equals(3, service.UnreadCount)
	Assert.Equals(3, service.Tabs[0].UnreadCount)
	Assert.Equals(1, service.Tabs[1].UnreadCount)
	Assert.IsTrue(!service.Tabs[3].HasUnread)
}

func (test *NotificationServiceTest) TestLoadWaitsWhileNobodyIsSignedIn() {
	service := NotificationTestSetup{}.list(test.behaviors, NotificationTestSetup{}.signedOutAccess(test.behaviors))
	service.Load(NotificationLoadRequested{})
	Assert.Equals("waiting", service.Status)
}

func (test *NotificationServiceTest) TestLoadFailureShowsTheErrorState() {
	service := test.build("broken.example")
	service.Load(NotificationLoadRequested{})
	Assert.Equals("error", service.Status)
}

func (test *NotificationServiceTest) TestReloadShowsTheErrorState() {
	service := test.build("broken.example")
	service.Start()
	service.Reload()
	Assert.Equals("error", service.Status)
}

func (test *NotificationServiceTest) TestReloadReadsTheDevicePermission() {
	service := test.loaded()
	Assert.Equals("on", service.push.Panel)
}

func (test *NotificationServiceTest) TestChooseTabLoadsWhenNothingIsLoaded() {
	service := test.build("example.com")
	service.Start()
	service.ChooseTab("dm")
	Assert.Equals("ready", service.Status)
	Assert.Equals(1, len(service.Days))
}

func (test *NotificationServiceTest) TestChooseTabShowsOnlyThatKind() {
	service := test.loaded()
	service.ChooseTab("email")
	Assert.Equals(1, len(service.Days))
	Assert.Equals("Yesterday", service.Days[0].Label)
	Assert.Equals("activity-tab-email", service.TabId)
	Assert.IsTrue(service.Tabs[4].Selected)
}

func (test *NotificationServiceTest) TestLoadMoreAddsTheNextPage() {
	service := test.loaded()
	service.LoadMore()
	Assert.Equals("", service.NextCursor)
	Assert.IsTrue(!service.HasMore)
	Assert.Equals(3, len(service.Days))
	Assert.Equals("Tuesday 3 November", service.Days[2].Label)
}

func (test *NotificationServiceTest) TestLoadMoreKeepsTheCursorWhenItFails() {
	service := test.loaded()
	service.NextCursor = "unknown"
	service.LoadMore()
	Assert.Equals("unknown", service.NextCursor)
	Assert.Equals("error", service.ActionProblem)
}

func (test *NotificationServiceTest) TestOpenMarksTheRowRead() {
	service := test.loaded()
	service.Open(service.Days[0].Rows[0])
	Assert.Equals(2, service.UnreadCount)
	Assert.IsTrue(!service.Days[0].Rows[0].Unread)
	Assert.Equals("", service.ActionProblem)
}

func (test *NotificationServiceTest) TestOpenAReadRowSendsNothing() {
	service := test.loaded()
	service.Open(service.Days[0].Rows[2])
	Assert.Equals(3, service.UnreadCount)
	Assert.Equals("", service.ActionProblem)
}

func (test *NotificationServiceTest) TestMarkReadMarksOneRow() {
	service := test.loaded()
	service.MarkRead("n2")
	Assert.Equals(2, service.UnreadCount)
}

func (test *NotificationServiceTest) TestMarkReadKeepsTheRowUnreadWhenItFails() {
	service := test.loaded()
	service.MarkRead("n9")
	Assert.Equals(3, service.UnreadCount)
	Assert.Equals("error", service.ActionProblem)
}

func (test *NotificationServiceTest) TestMarkAllReadMarksEveryUnreadRow() {
	service := test.loaded()
	service.MarkAllRead()
	Assert.Equals(0, service.UnreadCount)
	Assert.Equals("", service.ActionProblem)
}

func (test *NotificationServiceTest) TestDomainChangedClearsTheOldDomain() {
	service := test.loaded()
	service.ChooseTab("email")
	service.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	Assert.Equals("loading", service.Status)
	Assert.Equals("all", service.Tab)
	Assert.IsTrue(!service.known)
	Assert.Equals(0, service.UnreadCount)
	service.Reload()
	Assert.Equals(3, service.UnreadCount)
}

func (test *NotificationServiceTest) TestNewItemWhileHiddenShowsOneSystemNotification() {
	service := test.loaded()
	delete(service.seen, "n1")
	BrowserFakes{}.Answer("pageVisibility", "visible", false)
	service.Load(NotificationLoadRequested{})
	Assert.Equals(map[string]bool{"n1": true}, service.push.shown)
}

func (test *NotificationServiceTest) TestFirstLoadShowsNoSystemNotification() {
	service := test.build("example.com")
	BrowserFakes{}.Answer("pageVisibility", "visible", false)
	service.Load(NotificationLoadRequested{})
	Assert.Equals(0, len(service.push.shown))
}

func (test *NotificationServiceTest) TestNewItemDuringDoNotDisturbShowsNoSystemNotification() {
	service := test.loaded()
	service.push.presence.DoNotDisturb("1h")
	delete(service.seen, "n1")
	BrowserFakes{}.Answer("pageVisibility", "visible", false)
	service.Load(NotificationLoadRequested{})
	Assert.Equals(0, len(service.push.shown))
}

func (test *NotificationServiceTest) TestNewItemWhileVisibleShowsNoSystemNotification() {
	service := test.loaded()
	delete(service.seen, "n1")
	service.Load(NotificationLoadRequested{})
	Assert.Equals(0, len(service.push.shown))
}
