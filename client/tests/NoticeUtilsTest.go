// NoticeUtilsTest covers the notification centre's notices.
// @Test(ShellTestSetup, {"partition":"tests"})
type NoticeUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *NoticeUtilsTest) TestNoticesComeHighestPriorityFirst() {
	notices := NoticeUtils{}.Build(NoticeState{Online: false, UpdateVersion: "2.0", AccountErrors: []string{"a.com"}, DisabledRuleCount: 2, PushPrompt: true})
	Assert.Equals([]string{"offline", "update-2.0", "account-error", "disabled-rules", "push-prompt"}, []string{notices[0].ID, notices[1].ID, notices[2].ID, notices[3].ID, notices[4].ID})
	Assert.Equals("Mail updates for a.com are reconnecting.", notices[2].Title)
	Assert.Equals("2 mail rules are disabled.", notices[3].Title)
}

func (test *NoticeUtilsTest) TestOnlyOptionalNoticesCanBeDismissed() {
	notices := NoticeUtils{}.Visible(NoticeUtils{}.Build(NoticeState{Online: false, PushPrompt: true}), []string{"push-prompt", "offline"})
	Assert.Equals(1, len(notices))
	Assert.Equals("shell-notification shell-notification-offline highest-priority", notices[0].Classes)
	Assert.Equals(false, NoticeUtils{}.Dismissable(notices, "offline"))
}

func (test *NoticeUtilsTest) TestButtonLabelCountsTheNotices() {
	Assert.Equals("Notifications, none active", NoticeUtils{}.Label(0))
	Assert.Equals("Notifications, 3 active", NoticeUtils{}.Label(3))
}

func (test *NoticeUtilsTest) TestPushPromptNeedsAnUnaskedBrowserWithoutSubscription() {
	features := BrowserFeatures{Notifications: true}
	Assert.IsTrue(NoticeUtils{}.PushPrompt(features, "default", true, 0))
	Assert.Equals(false, NoticeUtils{}.PushPrompt(features, "granted", true, 0))
	Assert.Equals(false, NoticeUtils{}.PushPrompt(BrowserFeatures{Notifications: true, PushDisabled: true}, "default", true, 0))
}
