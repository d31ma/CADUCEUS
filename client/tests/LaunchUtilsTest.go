// LaunchUtilsTest covers the installed app's launch prefill and badge.
// @Test(ShellTestSetup, {"partition":"tests"})
type LaunchUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *LaunchUtilsTest) TestMailtoLaunchFillsTheComposer() {
	Assert.Equals(map[string]any{"to": "ada@x.com, bob@x.com", "subject": "Hello", "body": "Hi there"},
		LaunchUtils{}.ComposePrefill("", "?protocol="+"mailto%3Aada%40x.com%2Cbob%40x.com%3Fsubject%3DHello%26body%3DHi%2520there"))
}

func (test *LaunchUtilsTest) TestShareTargetKeepsOnlyWebAddresses() {
	Assert.Equals(map[string]any{"subject": "Read this", "body": "Look\n\nhttps://example.com/a"}, LaunchUtils{}.ComposePrefill("", "shareTitle=Read+this&shareText=Look&shareUrl=https%3A%2F%2Fexample.com%2Fa"))
	Assert.Equals(map[string]any{"body": "Look", "to": "kept@x.com"}, LaunchUtils{}.ComposePrefill(`{"to":"kept@x.com"}`, "shareText=Look&shareUrl=javascript%3Aalert(1)"))
	Assert.IsTrue(LaunchUtils{}.IsComposeLaunch("?shareText=x"))
	Assert.Equals(false, LaunchUtils{}.IsComposeLaunch("?to=ada@x.com"))
}

func (test *LaunchUtilsTest) TestBadgeFollowsTheChosenScope() {
	counted := FolderCountsChanged{UnreadInbox: 7, UnreadInboxByDomain: map[string]int{"a.com": 3}}
	Assert.Equals(7, MailboxUtils{}.BadgeCount(counted, BadgePreference{}, []string{"a.com"}))
	Assert.Equals(3, MailboxUtils{}.BadgeCount(counted, BadgePreference{Scope: "domain", Domain: "A.com"}, []string{"a.com"}))
	Assert.Equals(7, MailboxUtils{}.BadgeCount(counted, BadgePreference{Scope: "domain", Domain: "b.com"}, []string{"a.com"}))
}

func (test *LaunchUtilsTest) TestNotificationButtons() {
	patch, message := MailboxUtils{}.NotificationAction("archive")
	Assert.Equals(map[string]any{"folder": "archive"}, patch)
	Assert.Equals("Archived from notification.", message)
	Assert.IsTrue(MailboxUtils{}.SafeID("msg_1:2"))
	Assert.Equals(false, MailboxUtils{}.SafeID("../x y"))
}
