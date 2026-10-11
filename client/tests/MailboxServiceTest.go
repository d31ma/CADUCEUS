// MailboxServiceTest covers the sidebar badges and labels.
// @Test(ShellTestSetup, {"partition":"tests"})
type MailboxServiceTest struct {
	behaviors *BehaviorContext
}

func (test *MailboxServiceTest) mailbox() *MailboxService {
	mailbox := &MailboxService{}
	mailbox.Start()
	mailbox.HearCounts(FolderCountsChanged{
		Counts:    map[string]int{"inbox": 3, "archive": 2, "clients": 1},
		ByDomain:  map[string]map[string]int{"example.com": {"inbox": 2, "clients": 1}},
		ByAccount: map[string]map[string]int{"ext_1": {"inbox": 4}},
	})
	return mailbox
}

func (test *MailboxServiceTest) TestOnlyTheInboxShowsABadgeByDefault() {
	mailbox := test.mailbox()
	Assert.Equals("3", mailbox.FolderCount("inbox"))
	Assert.Equals("", mailbox.FolderCount("archive"))
	Assert.Equals("2", mailbox.DomainFolderCount("inbox", "example.com"))
	Assert.Equals("4", mailbox.AccountFolderCount("inbox", "ext_1"))
}

func (test *MailboxServiceTest) TestEveryFolderShowsABadgeWhenAsked() {
	mailbox := test.mailbox()
	mailbox.HearSession(SessionChanged{SignedIn: false, Domains: []string{"example.com"}})
	mailbox.HearPreferences(PreferencesLoaded{ShowUnreadForAllFolders: true, MailboxNames: map[string]string{"Example.com": "Work"}})
	Assert.Equals("2", mailbox.FolderCount("archive"))
	Assert.Equals("Work", mailbox.MailboxLabel("EXAMPLE.com"))
	Assert.Equals("other.com", mailbox.MailboxLabel("Other.com"))
	Assert.Equals([]string{"clients"}, mailbox.CustomFoldersForDomain("example.com"))
	Assert.Equals("Clients / Acme", mailbox.FolderLabel("clients/acme"))
}

func (test *MailboxServiceTest) TestActivityViewedClearsTheBadge() {
	mailbox := test.mailbox()
	mailbox.HearStorage(StoredValues{Session: map[string]string{}, Local: map[string]string{}})
	mailbox.HearActivityViewed(ActivityViewed{})
	Assert.Equals(0, mailbox.Counts["activity"])
	mailbox.HearTimer(TimerElapsed{Name: "other"})
	mailbox.HearRefresh(InboxRefreshRequested{})
	mailbox.HearToastAction(ToastActionChosen{Action: "nothing"})
}
