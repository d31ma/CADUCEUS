import (
	"strings"
	"time"
)

// DetailServicesTest covers the message reader's Go services, with the
// requests answered by DetailTestSetup.
// @Test(DetailTestSetup, {"partition":"tests"})
type DetailServicesTest struct {
	behaviors *BehaviorContext
	// @Fake
	inboxClient *InboxClient
	// @Fake
	usersClient *UsersClient
	// @Fake
	threadsClient *ThreadsClient
	// @Fake
	contactsClient *ContactsClient
	// @Fake
	searchClient *SearchClient
	// @Fake
	githubClient *GithubClient
	// @Fake
	snoozeClient *SnoozeClient
	// @Fake
	calendarClient *CalendarClient
	// @Fake
	threadSharesClient *ThreadSharesClient
}

func (test *DetailServicesTest) openedMessage(path string) *MessageService {
	message := &MessageService{}
	test.construct(message)
	message.Start()
	message.HearStorage(StoredValues{Session: map[string]string{"caduceus_mail_preferences": `{"markAsReadDelay":-1,"detailedHeaders":true,"detailedNames":true}`}, Local: map[string]string{}})
	message.HearSession(SessionChanged{SignedIn: true, Token: "t", Email: "Owner@Example.com"})
	message.HearLocation(PageLocation{Path: path})
	return message
}

func (test *DetailServicesTest) opened() OpenMessageChanged {
	return OpenMessageChanged{EmailId: "m1", Folder: "inbox", ParticipantAddress: "grace@example.net", Standalone: true, Account: "owner@example.com"}
}

func (test *DetailServicesTest) TestAMessagePageOpensItsMessage() {
	message := test.openedMessage("/email/m1")
	Assert.IsTrue(message.HasEmail)
	Assert.IsTrue(message.Standalone)
	Assert.Equals("", message.LoadError)
	Assert.Equals("Plans m1", message.Email.Subject)
	Assert.Equals("Grace Hopper <grace@example.net>", message.VisibleSender)
	Assert.Equals("grace@example.net", message.ReplyTarget)
	Assert.Equals("grace@example.net", message.ParticipantAddress)
	Assert.Equals("html", message.RenderMode)
	Assert.IsTrue(message.CanArchive)
	Assert.IsTrue(message.CanSnooze)
	Assert.Equals(false, message.CanMoveToInbox)
	Assert.Equals(false, message.CanDeleteForever)
	Assert.IsTrue(message.HasAttachments)
	Assert.Equals("2.0 KB", message.Email.Attachments[0].SizeLabel)
	Assert.IsTrue(message.HasDetailedHeaders)
	Assert.Equals("team@example.com", message.DetailedHeaderRows[2].Value)
	Assert.IsTrue(message.Email.ReceivedLabel != "")
}

func (test *DetailServicesTest) TestRemoteImagesStayBlockedUntilAsked() {
	message := test.openedMessage("/email/m1")
	Assert.Equals(1, message.BlockedRemoteContentCount)
	Assert.IsTrue(message.RemoteContentSummary != "")
	message.ShowRemoteImages()
	Assert.Equals(0, message.BlockedRemoteContentCount)
	Assert.Equals("", message.RemoteImageError)
	message.AlwaysShowImagesFromSender()
	Assert.Equals(false, message.RemotePreferenceSaving)
}

func (test *DetailServicesTest) TestTheMessageListOpensASelectedMessage() {
	message := test.openedMessage("/inbox")
	Assert.Equals(false, message.HasEmail)
	message.HearSelection(MailSelected{EmailID: "m1"})
	Assert.IsTrue(message.HasEmail)
	Assert.Equals(false, message.Standalone)
	message.GoBack()
}

func (test *DetailServicesTest) TestToolbarActionsShowAtOnce() {
	message := test.openedMessage("/email/m1")
	message.ToggleStar()
	Assert.IsTrue(message.Email.Starred)
	message.ToggleRead()
	Assert.IsTrue(message.Email.Read)
	message.Archive()
	Assert.Equals("archive", message.Email.Folder)
	Assert.Equals(false, message.CanArchive)
	Assert.IsTrue(message.CanMoveToInbox)
	message.MoveToTrash()
	Assert.IsTrue(message.CanDeleteForever)
	message.ReportSpam()
	message.OpenPicker("move")
	message.HearShortcut(ShortcutPressed{Command: "email:star"})
	Assert.Equals(false, message.Email.Starred)
}

func (test *DetailServicesTest) TestRenderModesSourceAndAttachments() {
	message := test.openedMessage("/email/m1")
	message.SetRenderMode("text")
	Assert.Equals("Hello there", message.VisiblePlainText)
	Assert.IsTrue(message.HasQuotedText)
	message.SetRenderMode("source")
	Assert.Equals("source", message.RenderMode)
	Assert.Equals("Subject: Plans\r\n\r\nHello", message.RawSourceText)
	message.DownloadOriginal()
	Assert.Equals("m1.eml", message.DownloadName)
	message.PreviewAttachment("a1")
	Assert.IsTrue(message.AttachmentPreviewOpen)
	Assert.Equals("hello notes", message.AttachmentPreviewText)
	message.HearBackdrop(BackdropClicked{Dialog: "reader-attachment"})
	Assert.Equals(false, message.AttachmentPreviewOpen)
	message.DownloadAttachment("a1")
	Assert.Equals("data:text/plain;base64,aGVsbG8gbm90ZXM=", message.DownloadUrl)
}

func (test *DetailServicesTest) TestMarkReadRepliesAndDelete() {
	message := test.openedMessage("/email/m1")
	message.HearTimer(TimerElapsed{Name: "reader-mark-read"})
	Assert.IsTrue(message.Email.Read)
	message.PreferredReply()
	message.ReplyAll()
	message.Forward()
	message.HearConfirmation(ConfirmationGiven{Action: "reader-delete", ID: "m1"})
	Assert.Equals(false, message.HasEmail)
}

func (test *DetailServicesTest) TestLinkWarningsAndTranslation() {
	message := test.openedMessage("/email/m1")
	message.HearLinkWarning(PendingLinkWarning{Reason: "Looks like a.example", Target: "https://b.example/x", DisplayedHost: "a.example"})
	Assert.IsTrue(message.HasPendingLink)
	Assert.Equals("b.example", message.PendingLink.DestinationHost)
	message.HearBackdrop(BackdropClicked{Dialog: "reader-link"})
	Assert.Equals(false, message.HasPendingLink)
	message.HearDetection(TranslationDetected{MessageId: "m1", Language: "es", Confidence: 0.9, Preference: "offer"})
	Assert.IsTrue(message.TranslationOfferVisible)
	message.TranslateMessage()
	Assert.IsTrue(message.TranslationBusy)
	message.HearTranslationPart(TranslationPart{MessageId: "m1", Kind: "text", Index: 0, Count: 1, Text: "Hola"})
	message.HearTranslationPart(TranslationPart{MessageId: "m1", Kind: "done", Index: 0, Count: 1, Text: "es"})
	Assert.IsTrue(message.TranslationVisible)
	Assert.Equals(false, message.TranslationBusy)
	message.SetRenderMode("text")
	Assert.Equals("Hola", message.VisiblePlainText)
	message.ShowOriginalMessage()
	Assert.Equals("Hello there", message.VisiblePlainText)
	message.HearTranslationFailure(TranslationFailed{MessageId: "m1"})
	Assert.Equals("Translation failed.", message.TranslationError)
}

func (test *DetailServicesTest) TestTheConversationAndFind() {
	conversation := &ConversationService{}
	test.construct(conversation)
	conversation.Start()
	conversation.HearSession(SessionChanged{Token: "t"})
	conversation.HearMessage(test.opened())
	Assert.Equals(2, len(conversation.Messages))
	Assert.Equals("m1", conversation.Messages[0].Id)
	Assert.IsTrue(conversation.HasConversation)
	Assert.Equals("2 messages", conversation.CountLabel)
	Assert.IsTrue(conversation.Messages[0].Expanded)
	conversation.ToggleQuote("m1")
	Assert.IsTrue(conversation.Messages[0].QuoteExpanded)
	conversation.OpenFind()
	conversation.FindInput("hello")
	Assert.Equals(2, conversation.FindCount)
	Assert.Equals("1 of 2", conversation.FindProgressLabel)
	conversation.FindKey("Enter", false)
	Assert.Equals("2 of 2", conversation.FindProgressLabel)
	conversation.FindKey("Escape", false)
	Assert.Equals(false, conversation.FindOpen)
	conversation.Reply("m2")
	conversation.OpenMessage("m2")
}

func (test *DetailServicesTest) TestTheContactSidebar() {
	participant := &ParticipantService{}
	test.construct(participant)
	participant.HearSession(SessionChanged{Token: "t"})
	participant.HearStorage(StoredValues{Local: map[string]string{"caduceus-participant-sidebar": "open"}})
	participant.HearMessage(test.opened())
	Assert.IsTrue(participant.HasContact)
	Assert.Equals("Grace Hopper", participant.DisplayName)
	Assert.Equals("found", participant.GithubStatus)
	Assert.Equals(1, len(participant.Threads))
	Assert.Equals("An earlier note", participant.Threads[0].Snippet)
	participant.Toggle()
	Assert.Equals("Show contact sidebar", participant.SidebarLabel)
}

func (test *DetailServicesTest) TestSnoozeSharingAndInvitations() {
	snooze := &SnoozeService{}
	test.construct(snooze)
	snooze.HearSession(SessionChanged{Token: "t"})
	snooze.HearMessage(test.opened())
	snooze.OpenDialog()
	Assert.IsTrue(snooze.Open)
	snooze.Custom = "2000-01-01T09:00"
	snooze.HearSubmit(FormSubmitted{Form: "reader-snooze"})
	Assert.Equals("Choose a future date and time.", snooze.Error)
	snooze.ChoosePreset("tomorrow", "Tomorrow")
	Assert.Equals(false, snooze.Open)
	Assert.Equals("", snooze.Error)
	snooze.HearToastAction(ToastActionChosen{Action: "reader-snooze-undo", Subject: "z1"})

	share := &ThreadShareService{}
	test.construct(share)
	share.Start()
	share.HearSession(SessionChanged{Token: "t"})
	share.HearOrigin(DevicePlatform{Origin: "https://mail.example.com"})
	share.HearMessage(test.opened())
	share.OpenDialog()
	Assert.Equals(2, len(share.Shares))
	Assert.IsTrue(share.Shares[0].Active)
	Assert.Equals("2 messages", share.Shares[0].MessageLabel)
	Assert.IsTrue(strings.HasPrefix(share.Shares[1].StatusLabel, "Revoked "))
	share.Create()
	Assert.Equals("https://mail.example.com/share/secret", share.Created.AbsoluteUrl)
	share.Revoke("s1")
	Assert.Equals("", share.Error)

	invite := &InviteService{}
	test.construct(invite)
	invite.HearSession(SessionChanged{Token: "t"})
	invite.HearMessage(OpenMessageChanged{EmailId: "m1", CalendarInvite: &CalendarInvite{Id: "ev1", Title: "Sync", Start: "2026-10-12T14:00:00Z", End: "2026-10-12T15:00:00Z", CanRespond: true}})
	Assert.IsTrue(invite.HasInvite)
	Assert.Equals("No response", invite.Invite.StatusLabel)
	invite.Respond("accepted")
	Assert.Equals("Going", invite.Invite.StatusLabel)
}

func (test *DetailServicesTest) TestSnoozeRequestIdentifiersDistinguishRepeatedChanges() {
	snooze := &SnoozeService{}
	snooze.HearMessage(test.opened())
	until := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	first := snooze.requestID(until, "Tomorrow")
	second := snooze.requestID(until, "Tomorrow")
	Assert.Equals(36, len(first))
	Assert.Equals(36, len(second))
	Assert.Equals("4", first[14:15])
	Assert.Equals(false, first == second)
}
