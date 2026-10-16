// FolderUtilsTest covers the sidebar's folder names and badges.
// @Test(ShellTestSetup, {"partition":"tests"})
type FolderUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *FolderUtilsTest) TestActiveFolderFollowsTheAddress() {
	Assert.Equals("archive", FolderUtils{}.Active("/folder/archive", "", "inbox"))
	Assert.Equals("snoozed", FolderUtils{}.Active("/inbox", "snoozed", "inbox"))
	Assert.Equals("inbox", FolderUtils{}.Active("/email/abc", "", "team"))
	Assert.Equals("team", FolderUtils{}.Active("/team", "", "inbox"))
	Assert.Equals("contacts", FolderUtils{}.Active("/unknown", "", "contacts"))
}

func (test *FolderUtilsTest) TestLabelTitleCasesEachPart() {
	Assert.Equals("Clients / Acme", FolderUtils{}.Label("clients/acme"))
	Assert.Equals("Folder", FolderUtils{}.Label(""))
}

func (test *FolderUtilsTest) TestCustomFoldersSkipSystemFolders() {
	Assert.Equals([]string{"clients", "travel"}, FolderUtils{}.Custom(map[string]int{"inbox": 2, "travel": 1, "clients": 4, "": 1}, []string{"inbox"}))
}

func (test *FolderUtilsTest) TestTallyCountsPerFolderDomainAndAccount() {
	counted := MailboxUtils{}.Tally([]MailSummary{
		{ID: "1", Folder: "inbox", Domain: "Example.com"},
		{ID: "2", Folder: "inbox", Domain: "example.com", Read: true},
		{ID: "3", SystemFolder: "archive", Folder: "x", Domain: "example.com"},
		{ID: "4", Folder: "inbox", ProviderAccountID: "ext_1"},
	})
	Assert.Equals(3, counted.Counts["inbox"])
	Assert.Equals(1, counted.Counts["archive"])
	Assert.Equals(2, counted.ByDomain["example.com"]["inbox"])
	Assert.Equals(1, counted.ByAccount["ext_1"]["inbox"])
	Assert.Equals(2, counted.UnreadInbox)
	Assert.Equals(1, counted.UnreadInboxByDomain["example.com"])
}

func (test *FolderUtilsTest) TestMailboxLabelsKeepOnlyTheUsersDomains() {
	Assert.Equals(map[string]string{"example.com": "Work"}, MailboxUtils{}.Names(`{"Example.com":" Work ","other.com":"Other"}`, []string{"example.com"}))
	Assert.Equals(`{"example.com":"Work"}`, MailboxUtils{}.Encoded(map[string]string{"example.com": "Work"}))
}

func (test *FolderUtilsTest) TestToastTexts() {
	Assert.Equals("Moved 2 messages to archive (example.com).", MailboxUtils{}.MovedMessage(2, "archive", "example.com"))
	Assert.Equals("Moved message to spam.", MailboxUtils{}.MovedMessage(1, "spam", ""))
	Assert.Equals("Reported spam.", MailboxUtils{}.SpamDone(true))
	Assert.Equals("Not spam failed.", MailboxUtils{}.SpamFailure(false))
	Assert.Equals(5, MailboxUtils{}.WithActivity(map[string]int{"inbox": 1}, 5)["activity"])
}
