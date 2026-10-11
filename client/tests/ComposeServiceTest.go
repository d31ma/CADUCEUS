import "strings"

// ComposeServiceTest covers the composer's Go service with the requests
// answered by ComposeTestSetup: opening a prefill or a draft, autosave and
// its conflict, recipient suggestions, attachment limits and sending.
// @Test(ComposeTestSetup, {"partition":"tests"})
type ComposeServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	contactsClient *ContactsClient
	// @Fake
	draftsClient *DraftsClient
	// @Fake
	sendClient *SendClient
}

func (test *ComposeServiceTest) opened(prefill string, launched map[string]any) *ComposeService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	BrowserFakes{}.Answer("clock", "timeZone", "Europe/London")
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-1")
	compose := &ComposeService{}
	test.construct(compose)
	compose.Start()
	compose.HearStorage(StoredValues{Session: map[string]string{"caduceus_compose_prefill": prefill}, Local: map[string]string{}})
	compose.HearLaunch(ComposeLaunched{Fields: launched})
	compose.HearSession(SessionChanged{SignedIn: true, Token: "t", Email: "owner@example.com"})
	return compose
}

func (test *ComposeServiceTest) ready(compose *ComposeService) *ComposeService {
	compose.HearSender(SenderChosen{Reason: "start", Address: "owner@example.com", Apply: true, NextCc: []string{"boss@example.com"}, Preferences: MailPreferencesUtils{}.Defaults()})
	return compose
}

func (test *ComposeServiceTest) TestTheComposerWaitsForTheSessionTheLaunchAndTheStorage() {
	compose := &ComposeService{}
	test.construct(compose)
	compose.Start()
	compose.HearSession(SessionChanged{SignedIn: true, Token: "t"})
	compose.HearLaunch(ComposeLaunched{Fields: map[string]any{"subject": "Hi"}})
	Assert.Equals("", compose.Subject)
	Assert.Equals("", compose.EditorDocument)
}

func (test *ComposeServiceTest) TestAReplyPrefillOpensWithItsBodyInTheEditor() {
	compose := test.opened(`{"to":"ada@example.com","subject":"Re: Plans","body":"<p>Quoted</p>","sourceEmailId":"e1"}`, map[string]any{})
	Assert.Equals("ada@example.com", compose.To)
	Assert.Equals("Re: Plans", compose.Subject)
	Assert.Equals("e1", compose.SourceEmailID)
	Assert.Equals("Europe/London", compose.ScheduleTimeZone)
	Assert.IsTrue(strings.Contains(compose.EditorDocument, "<p>Quoted</p>"))
	test.ready(compose)
	Assert.IsTrue(compose.Ready)
	Assert.Equals("boss@example.com", compose.Cc)
}

func (test *ComposeServiceTest) TestADraftPrefillLoadsTheSavedDraft() {
	compose := test.opened(`{"draftId":"d1"}`, map[string]any{})
	Assert.Equals("d1", compose.DraftID)
	Assert.Equals("ada@example.com", compose.To)
	Assert.Equals("desk@example.com", compose.ReplyTo)
	Assert.Equals("Saved", compose.DraftStatus)
	Assert.IsTrue(strings.Contains(compose.EditorDocument, "<p>Saved text</p>"))
}

func (test *ComposeServiceTest) TestAutosaveCreatesTheDraftOnce() {
	compose := test.ready(test.opened("", map[string]any{"to": "ada@example.com", "subject": "Hello"}))
	compose.HearSaveDue(DraftSaveDue{HTML: "<p>Body</p>", Text: "Body"})
	Assert.Equals("d9", compose.DraftID)
	Assert.Equals("Saved", compose.DraftStatus)
	compose.DraftStatus = ""
	compose.HearSaveDue(DraftSaveDue{HTML: "<p>Body</p>", Text: "Body"})
	Assert.Equals("Saved", compose.DraftStatus)
}

func (test *ComposeServiceTest) TestAStaleDraftReportsTheConflict() {
	compose := test.ready(test.opened(`{"draftId":"d1"}`, map[string]any{}))
	compose.DraftID = "d2"
	compose.HearSaveDue(DraftSaveDue{HTML: "<p>Changed</p>", Text: "Changed"})
	Assert.IsTrue(compose.DraftConflict)
	Assert.Equals("Newer saved version detected", compose.DraftStatus)
	compose.SaveDraftCopy()
	Assert.Equals("", compose.DraftID)
	Assert.Equals(false, compose.DraftConflict)
}

func (test *ComposeServiceTest) TestATooLargeBodyIsNotSaved() {
	compose := test.ready(test.opened("", map[string]any{"subject": "Hello"}))
	compose.HearSaveDue(DraftSaveDue{TooLarge: true})
	Assert.Equals(ComposeLimitUtils{}.TooLarge(), compose.DraftStatus)
	Assert.Equals("", compose.DraftID)
}

func (test *ComposeServiceTest) TestSuggestionsFillTheRecipientField() {
	compose := test.ready(test.opened("", map[string]any{}))
	compose.TypeRecipient("to", "gr")
	compose.HearRecipientQuery(RecipientQueryChanged{Field: "to", Value: "gr"})
	Assert.Equals(2, len(compose.RecipientOptions))
	Assert.Equals("Found in mail", compose.RecipientOptions[1].Detail)
	compose.RecipientKey("to", "ArrowDown")
	Assert.Equals(1, compose.RecipientIndex)
	compose.RecipientKey("to", "Enter")
	Assert.Equals("ops@example.net", compose.To)
	Assert.Equals(0, len(compose.RecipientOptions))
}

func (test *ComposeServiceTest) TestAttachmentsOverTheLimitAreRefused() {
	compose := test.ready(test.opened("", map[string]any{}))
	compose.HearOffer(AttachmentsOffered{Files: []AttachmentOffer{{Index: 0, Filename: "big.pdf", Size: 50000}}})
	compose.HearAdded(AttachmentsAdded{Attachments: []ComposeAttachment{{ID: "a1", Filename: "note.txt", Size: 12, ContentBase64: "aGVsbG8="}}})
	Assert.Equals(1, len(compose.Attachments))
	compose.RemoveAttachment("a1")
	Assert.Equals(0, len(compose.Attachments))
}

func (test *ComposeServiceTest) TestSendingNeedsARecipientAndASubject() {
	compose := test.ready(test.opened("", map[string]any{}))
	compose.Send()
	Assert.Equals("To and Subject are required.", compose.Error)
	Assert.Equals(false, compose.Loading)
	compose.To, compose.Subject = "ada@example.com", "Hi"
	compose.SendAndArchive()
	Assert.Equals("Send & Archive is available only from a source conversation.", compose.Error)
}

func (test *ComposeServiceTest) TestASentMessageLeavesTheComposer() {
	compose := test.ready(test.opened(`{"draftId":"d1"}`, map[string]any{}))
	compose.SendDefault()
	Assert.IsTrue(compose.Loading)
	compose.HearSendBody(SendBodyReady{HTML: "<p>Hi</p>", Text: "Hi", Accepted: true})
	Assert.Equals("", compose.Error)
	Assert.Equals("", compose.DraftID)
	compose.HearSaveDue(DraftSaveDue{HTML: "<p>Late</p>", Text: "Late"})
	Assert.Equals("", compose.DraftID)
}

func (test *ComposeServiceTest) TestARefusedSendShowsAFixedMessage() {
	compose := test.ready(test.opened("", map[string]any{"to": "refused@example.com", "subject": "Hi"}))
	compose.ScheduledEditID = "s1"
	compose.ScheduleAt("2999-01-01T08:00:00Z")
	compose.HearSendBody(SendBodyReady{HTML: "<p>Hi</p>", Text: "Hi", Accepted: true})
	Assert.Equals("Schedule failed.", compose.Error)
	Assert.Equals(false, compose.Loading)
	compose.HearSendBody(SendBodyReady{Accepted: false})
	Assert.Equals("Fill every highlighted template blank before sending.", compose.Error)
}

func (test *ComposeServiceTest) TestRemindersAndSchedulesMustBeInTheFuture() {
	compose := test.ready(test.opened("", map[string]any{}))
	compose.ChooseFollowUpReminder()
	Assert.IsTrue(compose.ReminderPickerOpen)
	compose.SetFollowUpReminder("1970-01-01T00:00:00Z")
	Assert.Equals("", compose.RemindAt)
	compose.SetFollowUpReminder("2999-01-01T09:00:00Z")
	Assert.Equals("2999-01-01T09:00:00Z", compose.RemindAt)
	Assert.Equals(false, compose.ReminderPickerOpen)
	compose.ChooseSendLater()
	Assert.Equals(false, compose.SchedulePickerOpen)
}
