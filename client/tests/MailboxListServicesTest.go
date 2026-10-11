// MailboxListServicesTest covers the Go services of the mailbox list and its
// pages, with the requests answered by MailboxListTestSetup.
// @Test(MailboxListTestSetup, {"partition":"tests"})
type MailboxListServicesTest struct {
	behaviors *BehaviorContext
	// @Fake
	inboxClient *InboxClient
	// @Fake
	threadsClient *ThreadsClient
	// @Fake
	searchClient *SearchClient
	// @Fake
	remindersClient *RemindersClient
	// @Fake
	sendClient *SendClient
	// @Fake
	activityClient *ActivityClient
	// @Fake
	categoriesClient *CategoriesClient
	// @Fake
	contactsClient *ContactsClient
}

func (test *MailboxListServicesTest) openedInbox() *InboxService {
	inbox := &InboxService{}
	test.construct(inbox)
	inbox.Start()
	inbox.SessionChanged(SessionChanged{SignedIn: true, Token: "t", Email: "Owner@Example.com", Domains: []string{"example.com"}})
	inbox.PageLocation(PageLocation{Path: "/inbox", Search: "?folder=inbox"})
	return inbox
}

func (test *MailboxListServicesTest) TestTheInboxLoadsAndDerivesItsRows() {
	inbox := test.openedInbox()
	Assert.IsTrue(inbox.Loaded)
	Assert.Equals(2, len(inbox.Rows))
	Assert.Equals(1, inbox.UnreadCount)
	Assert.Equals("Inbox", inbox.FolderTitle)
	Assert.Equals("inbox (2)", inbox.Folders[0].Label)
	Assert.Equals("archive (1)", inbox.Folders[1].Label)
	Assert.Equals("GR", inbox.Rows[0].Initials)
	Assert.Equals(0, inbox.Rows[0].TabIndex)
	Assert.Equals(-1, inbox.Rows[1].TabIndex)
}

func (test *MailboxListServicesTest) TestFoldersFiltersAndDensity() {
	inbox := test.openedInbox()
	inbox.SetFolder("Archive")
	Assert.Equals(1, len(inbox.Rows))
	inbox.SetStatusFilter("unread")
	Assert.Equals("unread", inbox.StatusFilter)
	inbox.ToggleDensity()
	Assert.Equals("compact", inbox.Density)
	Assert.Equals("Compact", inbox.DensityLabel)
	inbox.ChangeMailboxFilter("domain:example.com")
	Assert.Equals("domain:example.com", inbox.SelectedMailboxValue)
	inbox.ChangeMailboxFilter("domain:elsewhere.com")
	Assert.Equals("all", inbox.SelectedMailboxValue)
}

func (test *MailboxListServicesTest) TestSelectionAndBulkActions() {
	inbox := test.openedInbox()
	inbox.ToggleBulkSelection("one")
	Assert.Equals(1, inbox.SelectedCount)
	inbox.ToggleAllVisibleSelection()
	Assert.IsTrue(inbox.AllVisibleSelected)
	inbox.ToggleAllVisibleSelection()
	Assert.Equals(0, inbox.SelectedCount)
	inbox.ToggleBulkSelection("two")
	inbox.BulkArchive()
	Assert.Equals(0, inbox.SelectedCount)
	inbox.BulkTrash()
	inbox.BulkMarkRead()
	inbox.BulkMarkUnread()
	inbox.BulkStar()
	inbox.RemoveStars()
	inbox.OpenBulkMovePicker()
	inbox.OpenBulkLabelPicker()
	inbox.CategoryPickerApplied(CategoryPickerApplied{Mode: "move"})
	Assert.Equals(0, inbox.SelectedCount)
}

func (test *MailboxListServicesTest) TestQuickActionsAndRowActivation() {
	inbox := test.openedInbox()
	inbox.QuickStar("one")
	inbox.QuickArchive("one")
	inbox.QuickTrash("one")
	inbox.ReportSpam("one")
	inbox.MarkNotSpam("one")
	inbox.MailGesture(MailGesture{Kind: "swipe", EmailID: "one", Folder: "trash"})
	inbox.MailRowActivated(MailRowActivated{EmailID: "two"})
	Assert.Equals("two", inbox.SelectedID)
	inbox.MailRowFocused(MailRowFocused{ID: "one"})
	Assert.Equals(0, inbox.Rows[0].TabIndex)
	inbox.Compose()
	inbox.ShortcutPressed(ShortcutPressed{Command: "navigation:next-email"})
	Assert.Equals("two", inbox.SelectedID)
	inbox.ShortcutPressed(ShortcutPressed{Command: "core:deselect-all"})
	Assert.Equals("", inbox.SelectedID)
}

func (test *MailboxListServicesTest) TestTheRowMenu() {
	inbox := test.openedInbox()
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one", X: 20, Y: 30})
	Assert.IsTrue(inbox.ContextMenu.Open)
	Assert.Equals("Message actions", inbox.ContextMenu.Heading)
	Assert.Equals("Mark read", inbox.ContextMenu.ReadLabel)
	inbox.ContextToggleRead()
	Assert.Equals(false, inbox.ContextMenu.Open)
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one"})
	inbox.ContextToggleStar()
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one"})
	inbox.ContextArchive()
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one"})
	inbox.ContextTrash()
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one"})
	inbox.ContextMove()
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one"})
	inbox.ContextLabels()
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "one"})
	inbox.ContextMenuDismissed(ContextMenuDismissed{Key: "Escape"})
	Assert.Equals(false, inbox.ContextMenu.Open)
	inbox.ContextMenuRequested(ContextMenuRequested{EmailID: "two"})
	inbox.ContextOpen()
	Assert.Equals("two", inbox.SelectedID)
	inbox.CloseContextMenu()
}

func (test *MailboxListServicesTest) TestThreadsSearchAndNewMail() {
	inbox := test.openedInbox()
	inbox.ToggleThreadMode()
	Assert.Equals(1, len(inbox.Threads))
	Assert.Equals("Sent only to you", inbox.Threads[0].PersonalLevelLabel)
	inbox.MailRowActivated(MailRowActivated{ThreadID: "t1"})
	Assert.IsTrue(inbox.Threads[0].Expanded)
	inbox.ToggleThreadSelection("t1")
	Assert.IsTrue(inbox.Threads[0].Selected)
	inbox.ToggleThreadMode()
	inbox.SearchSubmitted(SearchSubmitted{Query: "from:grace"})
	Assert.Equals(1, len(inbox.Rows))
	inbox.SearchSubmitted(SearchSubmitted{ResetStatus: true})
	inbox.ListScrolled(200)
	inbox.NewMailArrived(NewMailArrived{Email: Email{ID: "new", Folder: "inbox", Domain: "example.com"}})
	Assert.Equals(1, inbox.NewMailCount)
	inbox.ShowNewMail()
	Assert.Equals(0, inbox.NewMailCount)
	inbox.InboxRefreshRequested(InboxRefreshRequested{Reason: "test"})
	inbox.OfflineSearchFinished(OfflineSearchFinished{Request: 0, Done: true})
	inbox.PreferencesLoaded(PreferencesLoaded{MailboxNames: map[string]string{"example.com": "Work"}})
	Assert.Equals("Work", inbox.MailboxOptions[0].Label)
	inbox.ExternalMailboxesChanged(ExternalMailboxesChanged{Accounts: []ExternalAccount{{ID: "ext_abcdefghijklmnopqrstu", Label: "Remote"}}})
	Assert.Equals("account:ext_abcdefghijklmnopqrstu", inbox.MailboxOptions[1].Value)
	inbox.Settled(ViewSettled{Source: "test"})
}

func (test *MailboxListServicesTest) TestTheSearchBox() {
	search := &ThreadSearchService{}
	test.construct(search)
	search.Start()
	search.SessionChanged(SessionChanged{SignedIn: true, Token: "t", Email: "owner@example.com"})
	search.MailboxScopeChanged(MailboxScopeChanged{Folder: "archive", StatusFilter: "unread", Domain: "all"})
	search.CategoryCatalogChanged(CategoryCatalogChanged{})
	search.ExternalMailboxesChanged(ExternalMailboxesChanged{})
	Assert.Equals("Search all mailboxes", search.Placeholder)
	search.Focus("", 0)
	Assert.Equals("is:unread in:archive ", search.Draft)
	search.Input("fr", 2, 0)
	Assert.IsTrue(search.Visible)
	Assert.Equals("from:", search.Suggestions[0].Title)
	search.SearchKeyPressed(SearchKeyPressed{Key: "Enter", Value: "fr", Caret: 2})
	Assert.Equals("from: ", search.Draft)
	search.SearchSuggestionsWanted(SearchSuggestionsWanted{Value: "from:grace", Caret: 10})
	Assert.Equals("from", search.Suggestions[0].Title[:4])
	search.SearchSuggestionChosen(SearchSuggestionChosen{Index: 0, Value: "from:grace", Caret: 10})
	search.Scrolled(12)
	Assert.Equals("transform:translateX(-12px)", search.LayerStyle)
	search.SearchKeyPressed(SearchKeyPressed{Key: "Escape", Value: "x", Caret: 1})
	search.SearchBlurred(SearchBlurred{})
	Assert.Equals(false, search.Visible)
	search.Submit()
	search.Clear()
	Assert.Equals("", search.Draft)
}

func (test *MailboxListServicesTest) TestTheCategoryPicker() {
	picker := &CategoryPickerService{}
	test.construct(picker)
	picker.SessionChanged(SessionChanged{SignedIn: true, Token: "t"})
	picker.PreferencesLoaded(PreferencesLoaded{MailboxNames: map[string]string{"example.com": "Work mail"}})
	picker.CategoryPickerRequested(CategoryPickerRequested{Mode: "label", Messages: []PickerMessage{{ID: "a", Domain: "example.com", Labels: []string{"work"}}}})
	Assert.IsTrue(picker.Open)
	Assert.Equals("Apply labels", picker.Title)
	Assert.Equals("Work mail", picker.DomainLabel)
	Assert.Equals("✓", picker.Items[0].Glyph)
	picker.SearchChanged("travel")
	Assert.IsTrue(picker.CanCreate)
	picker.Create()
	picker.CategoryPickerRequested(CategoryPickerRequested{Mode: "move", Messages: []PickerMessage{{ID: "a", Domain: "example.com", Folder: "inbox"}}})
	picker.Select("archive")
	Assert.Equals(false, picker.Open)
	picker.CategoryPickerRequested(CategoryPickerRequested{Mode: "move", Messages: []PickerMessage{{ID: "a", Domain: "a.com"}, {ID: "b", Domain: "b.com"}}})
	Assert.Equals(false, picker.Open)
	picker.CategoryPickerDismissed(CategoryPickerDismissed{Key: "Escape"})
	picker.Close()
}

func (test *MailboxListServicesTest) TestTheArchiveTools() {
	transfer := &MailboxTransferService{}
	test.construct(transfer)
	transfer.SessionChanged(SessionChanged{SignedIn: true, Token: "t"})
	transfer.MailboxScopeChanged(MailboxScopeChanged{Folder: "inbox", FolderTitle: "Inbox", Domain: "all", Domains: []string{"a.com", "b.com"}, Owner: "me@a.com", Total: 3})
	transfer.ChooseImportFiles()
	Assert.Equals("Choose one mailbox domain before importing EML files.", transfer.ImportError)
	transfer.ExportFolder("eml")
	Assert.Equals("Choose one mailbox domain before exporting individual EML files.", transfer.ExportError)
	transfer.ExportFolder("mbox")
	Assert.IsTrue(transfer.Exporting)
	transfer.ExportProgressed(ExportProgressed{Messages: 1, BytesLabel: "1.0 KB"})
	Assert.Equals("Exporting mbox: 1 of 3 messages · 1.0 KB", transfer.ExportProgressLabel)
	transfer.ExportFinished(ExportFinished{Messages: 2, SourceBytesLabel: "2.0 KB"})
	Assert.Equals("Exported 2 messages (2.0 KB source).", transfer.ExportNotice)
	transfer.CancelExport()
	transfer.MailboxScopeChanged(MailboxScopeChanged{Folder: "inbox", FolderTitle: "Inbox", Domain: "a.com", Domains: []string{"a.com"}, Owner: "me@a.com"})
	transfer.ChooseImportFiles()
	transfer.ImportProgressed(ImportProgressed{Imported: 0, Total: 2, Current: "a.eml"})
	Assert.Equals("0 of 2 imported · a.eml", transfer.ImportProgressLabel)
	transfer.ImportFinished(ImportFinished{Imported: 1, Failures: 1, FirstFailure: "b.eml: import failed"})
	Assert.Equals("Imported 1 message; 1 file failed. b.eml: import failed", transfer.ImportError)
	transfer.CancelImport()
}

func (test *MailboxListServicesTest) TestThePages() {
	reminders := &RemindersService{}
	test.construct(reminders)
	reminders.SessionChanged(SessionChanged{SignedIn: true, Token: "t"})
	Assert.Equals("10/13/2026", reminders.Items[0].RemindAtLabel)
	reminders.Open("e1")
	reminders.Dismiss("r1")
	reminders.Settled(ViewSettled{})
	scheduled := &ScheduledService{}
	test.construct(scheduled)
	scheduled.SessionChanged(SessionChanged{SignedIn: true, Token: "t"})
	Assert.Equals("a@example.com, b@example.com", scheduled.Items[0].RecipientLabel)
	Assert.Equals("Queued", scheduled.Items[0].StatusLabel)
	scheduled.Edit("s1")
	scheduled.Confirmed(ConfirmationGiven{Action: "cancel-scheduled", ID: "s1"})
	scheduled.Settled(ViewSettled{})
	activity := &ActivityService{}
	test.construct(activity)
	activity.SessionChanged(SessionChanged{SignedIn: true, Token: "t"})
	Assert.Equals("33.3%", activity.Metrics.TrackingRateLabel)
	Assert.Equals("100%", activity.Metrics.OpenRateLabel)
	Assert.Equals("example.com/a", activity.Events[0].TargetLabel)
	Assert.Equals("Recipient unavailable", activity.Events[0].RecipientLabel)
	Assert.Equals("height:100%", activity.Daily[0].SentStyle)
	Assert.Equals("50%", activity.Subjects[0].OpenRateLabel)
	activity.SetType("open")
	Assert.Equals("open events", activity.EventFilterLabel)
	activity.ChangeRange("7d")
	activity.Open("m1")
	activity.Settled(ViewSettled{})
	layout := &MailLayoutService{}
	layout.MailSelected(MailSelected{EmailID: "a"})
	Assert.Equals("a", layout.SelectedMail)
	layout.MailClosed(MailClosed{})
	Assert.Equals("drafts", layout.FolderFor("/folder/drafts"))
	Assert.Equals("", layout.FolderFor("/inbox"))
}
