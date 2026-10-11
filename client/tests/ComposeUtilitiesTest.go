import (
	"encoding/json"
	"errors"
	"strings"
)

// ComposeUtilitiesTest covers the composer's pure rules: how it opens, the
// autosave snapshot, the send request of each mode, the 64 KiB limits and the
// editor document.
// @Test(ShellTestSetup, {"partition":"tests"})
type ComposeUtilitiesTest struct {
	behaviors *BehaviorContext
}

func (test *ComposeUtilitiesTest) TestThePrefillMergesTheStoredFieldsWithTheLaunch() {
	prefill := DraftUtils{}.Prefill(`{"to":"ada@example.com","subject":"Stored","references":["<a@x>"]}`, map[string]any{"subject": "Launched"})
	Assert.Equals("ada@example.com", prefill.To)
	Assert.Equals("Launched", prefill.Subject)
	Assert.Equals("<a@x>", prefill.References[0])
	Assert.Equals("", DraftUtils{}.Prefill("not json", map[string]any{}).To)
}

func (test *ComposeUtilitiesTest) TestTheOpeningFollowsThePrefillThenThePointer() {
	Assert.Equals("draft", DraftUtils{}.Opening(ComposePrefill{DraftID: "d1", To: "a@x.com"}, DraftPointer{DraftID: "d2"}))
	Assert.Equals("prefill", DraftUtils{}.Opening(ComposePrefill{Subject: "Hi"}, DraftPointer{DraftID: "d2"}))
	Assert.Equals("recovered", DraftUtils{}.Opening(ComposePrefill{}, DraftPointer{DraftID: "d2"}))
	Assert.Equals("blank", DraftUtils{}.Opening(ComposePrefill{OpenTracking: true}, DraftPointer{}))
}

func (test *ComposeUtilitiesTest) TestASnapshotIsWorthSavingOnlyWhenItChanged() {
	snapshot := DraftUtils{}.Snapshot(MessageFields{To: "a@x.com, b@x.com", Subject: "Hi", HTML: "<p>x</p>"}, "client-1", 2)
	Assert.Equals(2, len(snapshot.To))
	Assert.Equals(DraftUtils{}.Fingerprint(snapshot), DraftUtils{}.Fingerprint(DraftUtils{}.Snapshot(MessageFields{To: "a@x.com,b@x.com", Subject: "Hi", HTML: "<p>x</p>"}, "client-1", 7)))
	Assert.Equals(false, DraftUtils{}.Worth(snapshot, DraftUtils{}.Fingerprint(snapshot), "d1"))
	Assert.IsTrue(DraftUtils{}.Worth(snapshot, "", ""))
	Assert.Equals(false, DraftUtils{}.Worth(DraftUtils{}.Snapshot(MessageFields{HTML: "  "}, "c", 0), "", ""))
	Assert.IsTrue(DraftUtils{}.Worth(DraftUtils{}.Snapshot(MessageFields{}, "c", 0), "", "d1"))
}

func (test *ComposeUtilitiesTest) TestADraftGivesBackItsFields() {
	fields := DraftUtils{}.Fields(Draft{Sender: "me@x.com", To: []string{"a@x.com", "b@x.com"}, ReplyTo: []string{"desk@x.com"}, Body: "text"})
	Assert.Equals("a@x.com, b@x.com", fields.To)
	Assert.Equals("desk@x.com", fields.ReplyTo)
	Assert.Equals("text", fields.HTML)
	Assert.Equals("", DraftUtils{}.Fields(Draft{}).ReplyTo)
}

func (test *ComposeUtilitiesTest) TestTheDraftPointerIsStoredAndForgotten() {
	stored := DraftUtils{}.Stored(DraftPointer{DraftID: "d1", ClientID: "c1", Version: 2})
	Assert.Equals("session", stored.Area)
	Assert.Equals("d1", DraftUtils{}.Pointer(stored.Values["caduceus_compose_draft"]).DraftID)
	Assert.Equals("caduceus_compose_draft", DraftUtils{}.Forgotten().Removed[0])
}

func (test *ComposeUtilitiesTest) TestEachModeShapesTheSendRequest() {
	fields := MessageFields{From: "me@x.com", To: "a@x.com", ReplyTo: "r1@x.com, r2@x.com", Subject: "Hi", HTML: "<p>Hi</p>", Text: "Hi", SourceEmailID: "e1"}
	archived := SendUtils{}.Input(fields, SendChoice{Mode: "archive", DelayMs: 5000, RichText: true, IdempotencyKey: "send:1"})
	Assert.IsTrue(archived.Archive)
	Assert.Equals("e1", archived.ArchiveSourceID)
	Assert.Equals(5000, archived.DelayMs)
	Assert.Equals("r1@x.com, r2@x.com", archived.ReplyTo[0])
	scheduled := SendUtils{}.Input(fields, SendChoice{Mode: "schedule-edit", SendAt: "2030-01-01T08:00:00Z", DelayMs: 5000})
	Assert.Equals("2030-01-01T08:00:00Z", scheduled.SendAt)
	Assert.Equals(0, scheduled.DelayMs)
	Assert.Equals("", scheduled.HTML)
	encoded, _ := json.Marshal(scheduled)
	Assert.IsTrue(strings.Contains(string(encoded), `"remindAt":null`))
	Assert.Equals(false, strings.Contains(string(encoded), `"archive"`))
}

func (test *ComposeUtilitiesTest) TestTheSendOutcomeNamesItsPageAndNotice() {
	Assert.Equals("/scheduled", SendUtils{}.Route("schedule"))
	Assert.Equals("/inbox", SendUtils{}.Route("archive"))
	Assert.Equals("Schedule failed.", SendUtils{}.Failure("schedule-edit", errors.New("HTTP_STATUS: x")))
	Assert.Equals("Network error.", SendUtils{}.Failure("send", errors.New("HTTP_NETWORK: x")))
	Assert.Equals("Message sent.", SendUtils{}.Sent("send", SendAnswer{}))
	Assert.Equals("Sent & archived.", SendUtils{}.Sent("archive", SendAnswer{Archived: true}))
	Assert.Equals("Message sent; source was not archived.", SendUtils{}.Sent("archive", SendAnswer{}))
	Assert.Equals(false, SendUtils{}.Sounded("schedule"))
}

func (test *ComposeUtilitiesTest) TestAttachmentsMustFitTheTransport() {
	Assert.Equals("", ComposeLimitUtils{}.Problem(AttachmentOffer{Filename: "a.txt", Size: 1024}, 0, 0))
	Assert.IsTrue(strings.Contains(ComposeLimitUtils{}.Problem(AttachmentOffer{Filename: "big.pdf", Size: 40 * 1024}, 0, 0), "big.pdf is too large"))
	Assert.Equals("You can attach up to 10 files.", ComposeLimitUtils{}.Problem(AttachmentOffer{Size: 1}, 10, 0))
	Assert.Equals("a.txt is not a supported inline image.", ComposeLimitUtils{}.Problem(AttachmentOffer{Filename: "a.txt", Size: 1, Inline: true, ContentType: "text/plain"}, 0, 0))
	Assert.Equals(ComposeLimitUtils{}.TooLarge(), ComposeLimitUtils{}.RequestProblem(SendInput{Text: strings.Repeat("x", 70*1024)}))
	Assert.Equals("", ComposeLimitUtils{}.DraftProblem(DraftInput{Text: "small"}))
	Assert.Equals("Fill every highlighted template blank before sending.", ComposeLimitUtils{}.BodyProblem(SendBodyReady{}))
	Assert.Equals("", ComposeLimitUtils{}.BodyProblem(SendBodyReady{Accepted: true}))
}

func (test *ComposeUtilitiesTest) TestTheEditorDocumentShowsInlineImages() {
	image := ComposeAttachment{ID: "a1", ContentID: "caduceus.1@inline", ContentType: "image/png", ContentBase64: "iVBO"}
	document := EditorUtils{}.Document(`<p>Hi<img src="cid:caduceus.1@inline"><img src='cid:gone@x'></p>`, []ComposeAttachment{image}, 7)
	Assert.IsTrue(strings.Contains(document, `content="7"`))
	Assert.IsTrue(strings.Contains(document, `src="data:image/png;base64,iVBO" data-caduceus-cid="caduceus.1@inline" data-caduceus-attachment-id="a1"`))
	Assert.IsTrue(strings.Contains(document, `#missing-inline-image" data-caduceus-cid="gone@x"`))
	Assert.IsTrue(strings.Contains(document, `<body contenteditable="true" data-compose-editor`))
	Assert.Equals("caduceus.a-1.b@inline", EditorUtils{}.InlineContentID(true, "a-1.b/<>"))
	Assert.Equals("", EditorUtils{}.InlineContentID(false, "a1"))
}

func (test *ComposeUtilitiesTest) TestTheSignatureOfTheEditablePartCounts() {
	Assert.Equals("s1", SignatureUtils{}.CurrentID(`<p>Hi</p><signature id="s1" class="caduceus-signature">Ada</signature>`))
	Assert.Equals("s2", SignatureUtils{}.CurrentID(`<div class="caduceus-signature" data-signature-id="s2">Ada</div>`))
	Assert.Equals("", SignatureUtils{}.CurrentID(`<p>Hi</p><blockquote><signature id="s3">Old</signature></blockquote>`))
}

func (test *ComposeUtilitiesTest) TestSuggestionRowsDescribeTheirSource() {
	options := RecipientUtils{}.Options([]ContactSuggestion{{Email: "g@x.com", Name: "Grace Hopper"}, {Email: "o@x.com", Source: "mail"}, {Email: "c@x.com"}}, "to", 1)
	Assert.Equals("Grace Hopper", options[0].Label)
	Assert.Equals("g@x.com", options[0].Detail)
	Assert.Equals("GH", options[0].Initials)
	Assert.Equals("Found in mail", options[1].Detail)
	Assert.IsTrue(options[1].Active)
	Assert.Equals("compose-to-suggestions-2", options[2].OptionID)
	Assert.Equals("Contact", options[2].Detail)
}

func (test *ComposeUtilitiesTest) TestConnectedMailboxesThatSendBecomeFromChoices() {
	id := "ext_" + strings.Repeat("a", 24)
	accounts := []ExternalAccount{
		{ID: id + "2", MailboxAddress: "Ops@Example.net", Provider: "imap", State: "ready", Revision: 1, Order: 2, Capabilities: ExternalCapabilities{Send: true}},
		{ID: id + "1", MailboxAddress: "me@gmail.com", Label: "Personal", Provider: "gmail", State: "idle", Revision: 1, Order: 1, Capabilities: ExternalCapabilities{Send: true}},
		{ID: id + "3", MailboxAddress: "ops@example.net", Provider: "imap", State: "ready", Revision: 1, Order: 3, Capabilities: ExternalCapabilities{Send: true}},
		{ID: id + "4", MailboxAddress: "read@example.net", Provider: "imap", State: "ready", Revision: 1, Order: 4},
		{ID: id + "5", MailboxAddress: "paused@example.net", Provider: "imap", State: "paused", Revision: 1, Order: 5, Capabilities: ExternalCapabilities{Send: true}},
		{ID: "bad", MailboxAddress: "bad@example.net", Provider: "imap", State: "ready", Revision: 1, Capabilities: ExternalCapabilities{Send: true}},
	}
	identities := ExternalAccountUtils{}.SenderIdentities(accounts)
	Assert.Equals(2, len(identities))
	Assert.Equals("me@gmail.com", identities[0].Address)
	Assert.Equals("Personal", identities[0].Label)
	Assert.Equals("ops@example.net", identities[1].Address)
	Assert.Equals("Ops@Example.net", identities[1].Label)
	Assert.Equals(id+"2", identities[1].AccountID)
}
