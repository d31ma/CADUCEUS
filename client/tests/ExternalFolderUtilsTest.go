// ExternalFolderUtilsTest covers the connected-mailbox folder tree.
// @Test(ShellTestSetup, {"partition":"tests"})
type ExternalFolderUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *ExternalFolderUtilsTest) folders() []ExternalFolder {
	return ExternalFolderUtils{}.Normalize([]ExternalFolder{
		{StableID: "s-work", Name: "Work", Delimiter: "/"},
		{StableID: "s-inbox", Name: "INBOX", Role: "inbox", Delimiter: "/"},
		{StableID: "s-acme", Name: "Work/Acme", Delimiter: "/"},
		{StableID: "s-orphan", Name: "Gone/Child", Delimiter: "/"},
		{StableID: "", Name: "No id"},
		{StableID: "s-copy", Name: "work", Delimiter: "/"},
	}, "ext_account")
}

func (test *ExternalFolderUtilsTest) TestFoldersAreOrderedAsATree() {
	folders := test.folders()
	Assert.Equals([]string{"s-inbox", "s-orphan", "s-work", "s-acme"}, []string{folders[0].StableID, folders[1].StableID, folders[2].StableID, folders[3].StableID})
	Assert.Equals("Acme", folders[3].DisplayName)
	Assert.Equals(1, folders[3].Depth)
	Assert.IsTrue(folders[2].HasChildren)
}

func (test *ExternalFolderUtilsTest) TestChildrenShowOnlyUnderAnExpandedParent() {
	account := ExternalAccount{ID: "ext_account", State: "ready", Capabilities: ExternalCapabilities{MailWrite: true}}
	Assert.Equals(3, len(ExternalFolderUtils{}.Visible(test.folders(), map[string]bool{}, account)))
	open := map[string]bool{ExternalFolderUtils{}.OpenKey("ext_account", "s-work"): true}
	rows := ExternalFolderUtils{}.Visible(test.folders(), open, account)
	Assert.Equals(4, len(rows))
	Assert.Equals("/folder/work%2Facme?account=ext_account", rows[3].Href)
	Assert.Equals("--external-folder-depth:1", rows[3].RowStyle)
	Assert.Equals(FolderActions{CreateChild: true, Rename: true, Delete: true, Export: true}, rows[3].Actions)
	Assert.Equals(false, rows[0].Actions.Rename)
}

func (test *ExternalFolderUtilsTest) TestAccountsAreBoundedAndArranged() {
	accounts := ExternalAccountUtils{}.Normalize([]ExternalAccount{
		{ID: "ext_bbbbbbbbbbbbbbbbbbbbbbbb", MailboxAddress: "b@x.com", Provider: "imap", State: "ready", Revision: 1, Order: 2, Color: "red"},
		{ID: "ext_aaaaaaaaaaaaaaaaaaaaaaaa", MailboxAddress: "a@x.com", Provider: "gmail", State: "paused", Revision: 3, Order: 1, Label: "Personal"},
		{ID: "bad", MailboxAddress: "c@x.com", Provider: "imap", State: "ready", Revision: 1},
	})
	arranged := ExternalAccountUtils{}.Arranged(accounts)
	Assert.Equals(2, len(arranged))
	Assert.Equals("Personal", arranged[0].Label)
	Assert.Equals("#5f6368", arranged[1].Color)
	Assert.Equals(false, arranged[0].CanMoveUp)
	Assert.IsTrue(arranged[0].CanMoveDown)
	Assert.Equals(false, ExternalFolderUtils{}.Writable(arranged[0]))
	Assert.IsTrue(ExternalAccountUtils{}.Readable(ExternalAccount{State: "ready", Capabilities: ExternalCapabilities{MailRead: true}}))
	Assert.Equals("ext_bbbbbbbbbbbbbbbbbbbbbbbb", ExternalAccountUtils{}.Replaced(arranged, []ExternalAccount{{ID: "ext_bbbbbbbbbbbbbbbbbbbbbbbb", Order: 0, MailboxAddress: "b@x.com"}})[0].ID)
}

func (test *ExternalFolderUtilsTest) TestFolderDialog() {
	Assert.IsTrue(ExternalAccountUtils{}.ValidFolderName("Projects"))
	Assert.Equals(false, ExternalAccountUtils{}.ValidFolderName(" padded"))
	mutation := ExternalAccountUtils{}.Mutation(FolderDialog{Mode: "create", Folder: "s-work", Name: "New"}, 4, "key")
	Assert.Equals("s-work", mutation.ParentStableID)
	Assert.Equals("folder_create", mutation.Kind)
	Assert.Equals([]string{"Rename connected folder", "Choose a new name for “Work”.", "Working…"}, ExternalAccountUtils{}.DialogTexts(FolderDialog{Mode: "rename", Name: "Work"}, true, true))
}
