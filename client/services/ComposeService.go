import (
	"encoding/json"
	"slices"
	"time"
)

// ComposeService owns the message being written: it opens the prefill, the
// draft or the scheduled message the composer was launched with, keeps the
// header fields, recipient suggestions, attachments, reminder, schedule and
// tracking choices, autosaves the draft and sends. SenderService owns the From
// identity and the signatures, TemplateService the templates and
// GrammarService the grammar check. The live rich-text editor stays in
// ComposeEditorService (JavaScript): it hands the body over in events.
//
// The browser transport caps a request at 64 KiB, so a message with larger
// attachments has no route yet; the composer refuses them and says so.
//
// @Service
type ComposeService struct {
	// @Needs
	contactsClient *ContactsClient
	// @Needs
	draftsClient *DraftsClient
	// @Needs
	sendClient *SendClient
	// @Needs
	clock                     Clock
	applicationEventPublisher ApplicationEventPublisher

	To                 string
	Cc                 string
	Bcc                string
	ReplyTo            string
	Subject            string
	Loading            bool
	Ready              bool
	Error              string
	DraftID            string
	DraftStatus        string
	DraftConflict      bool
	EditorDocument     string
	EditorSnapshot     ComposeEditorSnapshot
	EditorBody         string
	EditorQuoteHTML    string
	EditorRevision     int
	RemindAt           string
	ReminderPickerOpen bool
	SchedulePickerOpen bool
	ScheduledEditID    string
	ScheduledSendAt    string
	SourceEmailID      string
	RequestReadReceipt bool
	TrackLinks         bool
	Attachments        []ComposeAttachment
	RecipientField     string
	RecipientOptions   []RecipientOption
	RecipientIndex     int
	CustomReminderAt   string
	CustomSendAt       string
	ScheduleTimeZone   string

	session          SessionChanged
	launch           ComposeLaunched
	stored           StoredValues
	launched         bool
	restored         bool
	opened           bool
	from             string
	html             string
	text             string
	inReplyTo        string
	references       []string
	clientID         string
	version          int
	saved            string
	firstSave        string
	generation       int
	quoteProof       *QuoteProof
	exiting          bool
	choice           SendChoice
	automaticCc      []string
	automaticBcc     []string
	preferences      MailPreferences
	suggestions      []ContactSuggestion
	recipientRequest int
}

// @OnStart
func (compose *ComposeService) Start() {
	compose.preferences = MailPreferenceUtils{}.Defaults()
	compose.Attachments = []ComposeAttachment{}
	compose.RecipientOptions = []RecipientOption{}
	compose.RecipientIndex = -1
	compose.ScheduleTimeZone = compose.timeZone()
	compose.firstSave = compose.names().Changed
}

// HearSession keeps the signed-in mailbox that every request names.
// @OnEvent({"replay":true})
func (compose *ComposeService) HearSession(changed SessionChanged) {
	compose.session = changed
	compose.openWhenReady()
}

// HearLaunch keeps the prefill that the shell hands the composer.
// @OnEvent({"replay":true})
func (compose *ComposeService) HearLaunch(launched ComposeLaunched) {
	compose.launch = launched
	compose.launched = true
	compose.openWhenReady()
}

// HearStorage reads the stored prefill and the draft this tab was writing.
// The shell removes the prefill key once it has published ComposeLaunched,
// so the composer reads it from this snapshot of the tab's storage.
// @OnEvent({"replay":true})
func (compose *ComposeService) HearStorage(stored StoredValues) {
	compose.stored = stored
	compose.restored = true
	compose.openWhenReady()
}

// HearSender follows the From identity that SenderService chose, merges its
// automatic CC and BCC recipients and, on start, makes the composer ready.
// @OnEvent({"replay":true})
func (compose *ComposeService) HearSender(chosen SenderChosen) {
	compose.from = chosen.Address
	compose.preferences = chosen.Preferences
	compose.mergeAutomatic(chosen)
	FlowUtils{}.Either(compose.starting(chosen.Reason), compose.becomeReady, func() { compose.saveSoon(compose.names().SaveNow) })()
}

// TypeRecipient stores a To, CC or BCC value as the person types.
func (compose *ComposeService) TypeRecipient(field string, value string) {
	compose.setRecipient(field, value)
}

// HearRecipientQuery loads suggestions for the address being typed. The
// editor service waits 140 ms after the last keystroke before it asks.
// @OnEvent
func (compose *ComposeService) HearRecipientQuery(changed RecipientQueryChanged) {
	compose.recipientRequest++
	request := compose.recipientRequest
	found := compose.suggestionsFor(RecipientUtils{}.CurrentToken(changed.Value))
	FlowUtils{}.When(request == compose.recipientRequest, func() { compose.showSuggestions(changed.Field, found) })()
	compose.publish(ComposeChanged{Reason: compose.names().Changed})
}

// HearRecipientsDismissed hides the list when focus leaves the fields.
// @OnEvent
func (compose *ComposeService) HearRecipientsDismissed(dismissed RecipientSuggestionsDismissed) {
	compose.hideSuggestions()
}

// RecipientKey moves through, picks or closes the suggestion list.
func (compose *ComposeService) RecipientKey(field string, key string) {
	FlowUtils{}.When(compose.listed(field), compose.suggestionKeys(key))()
}

// ChooseRecipient replaces the address being typed with a suggestion.
func (compose *ComposeService) ChooseRecipient(index int) {
	compose.chooseSuggestion(compose.suggestionAt(index))
}

// ChangeTracking records the open or link tracking choice.
func (compose *ComposeService) ChangeTracking(option string, enabled bool) {
	*map[string]*bool{compose.names().Open: &compose.RequestReadReceipt, "links": &compose.TrackLinks}[option] = enabled
	compose.saveSoon(compose.names().SaveNow)
}

// HearTemplate copies the template's recipients and subject once the editor
// has inserted its body.
// @OnEvent
func (compose *ComposeService) HearTemplate(inserted TemplateInserted) {
	compose.To = TextUtils{}.FirstOf(inserted.To, compose.To)
	compose.Cc = TextUtils{}.FirstOf(inserted.Cc, compose.Cc)
	compose.Subject = TextUtils{}.FirstOf(inserted.Subject, compose.Subject)
	compose.saveSoon(compose.names().SaveNow)
}

// HearEditorReplaced shows a body the editor rewrote as a whole, for example
// with a signature, a template or a translation.
// @OnEvent
func (compose *ComposeService) HearEditorReplaced(replaced EditorReplaced) {
	compose.html = replaced.HTML
	compose.showBody()
}

// HearOffer applies the attachment limits before the editor reads any file.
// @OnEvent
func (compose *ComposeService) HearOffer(offered AttachmentsOffered) {
	compose.publish(AttachmentsAccepted{Accepted: compose.acceptAll(offered.Files)})
}

// HearAdded lists the files the editor has read.
// @OnEvent
func (compose *ComposeService) HearAdded(added AttachmentsAdded) {
	compose.Attachments = append(append([]ComposeAttachment{}, compose.Attachments...), added.Attachments...)
	compose.saveSoon(compose.names().SaveNow)
}

// RemoveAttachment drops a pending attachment and its inline image.
func (compose *ComposeService) RemoveAttachment(id string) {
	removed := append(compose.attachmentsWhere(id, true), ComposeAttachment{})[0]
	compose.Attachments = compose.attachmentsWhere(id, false)
	compose.publish(AttachmentRemoved{ID: id, ContentID: removed.ContentID})
	compose.saveSoon(compose.names().SaveNow)
}

// HearImageRemoved drops the attachment of an inline image the person removed
// in the editor.
// @OnEvent
func (compose *ComposeService) HearImageRemoved(removed InlineImageRemoved) {
	compose.Attachments = compose.attachmentsWhere(removed.ID, false)
	compose.saveSoon(compose.names().SaveNow)
}

// AttachmentSize writes the size shown on an attachment pill.
func (compose *ComposeService) AttachmentSize(size int) string {
	return SizeUtils{}.Label(size)
}

// ChooseFollowUpReminder opens or closes the follow-up reminder picker.
func (compose *ComposeService) ChooseFollowUpReminder() {
	compose.ReminderPickerOpen = !compose.ReminderPickerOpen
}

// SetFollowUpReminder keeps a future reminder time chosen in the picker.
func (compose *ComposeService) SetFollowUpReminder(at string) {
	FlowUtils{}.Either(compose.isFuture(at), func() {
		compose.RemindAt, compose.ReminderPickerOpen = at, false
		compose.saveSoon(compose.names().SaveNow)
	}, func() { compose.toast("Please choose a future reminder time.") })()
}

// CloseFollowUpReminder closes the reminder picker.
func (compose *ComposeService) CloseFollowUpReminder() {
	compose.ReminderPickerOpen = false
}

// ClearFollowUpReminder removes the reminder.
func (compose *ComposeService) ClearFollowUpReminder() {
	compose.RemindAt, compose.ReminderPickerOpen = "", false
	compose.saveSoon(compose.names().SaveNow)
}

// ChooseSendLater opens the send-later picker once To and Subject are set.
func (compose *ComposeService) ChooseSendLater() {
	FlowUtils{}.When(compose.accepted(compose.problemFor(compose.names().Schedule)), func() {
		compose.SchedulePickerOpen, compose.ReminderPickerOpen = !compose.SchedulePickerOpen, false
	})()
}

// CloseSendLater closes the send-later picker.
func (compose *ComposeService) CloseSendLater() {
	compose.SchedulePickerOpen = false
}

// ScheduleAt schedules the message, or moves a scheduled one, to a future time.
func (compose *ComposeService) ScheduleAt(at string) {
	mode := map[bool]string{true: compose.names().ScheduleEdit, false: compose.names().Schedule}[compose.ScheduledEditID != ""]
	FlowUtils{}.Either(compose.isFuture(at), func() { compose.requestSend(SendChoice{Mode: mode, SendAt: at}) }, func() { compose.toast("Please choose a future time.") })()
}

// Send sends now; the server holds the message for the undo-send delay.
func (compose *ComposeService) Send() {
	compose.sendAs(compose.names().Send)
}

// SendAndArchive sends and archives the source conversation.
func (compose *ComposeService) SendAndArchive() {
	compose.sendAs(compose.names().Archive)
}

// SendDefault follows the default send action of the mail preferences.
func (compose *ComposeService) SendDefault() {
	compose.sendAs(compose.defaultMode())
}

// HearShortcut runs the composer's keyboard commands (Ctrl+Enter, Escape).
// @OnEvent
func (compose *ComposeService) HearShortcut(pressed ShortcutPressed) {
	FlowUtils{}.When(compose.Ready, compose.command(pressed.Command))()
	compose.publish(ComposeChanged{Reason: compose.names().Changed})
}

// HearSendBody sends the body the editor handed over for a send.
// @OnEvent
func (compose *ComposeService) HearSendBody(ready SendBodyReady) {
	problem := ComposeLimitUtils{}.BodyProblem(ready)
	FlowUtils{}.Either(problem == "", func() { compose.deliver(ready) }, func() { compose.Loading, compose.Error = false, problem })()
	compose.publish(ComposeChanged{Reason: compose.names().Changed})
}

// HearSaveDue autosaves once the editor's debounce timer fires.
// @OnEvent
func (compose *ComposeService) HearSaveDue(due DraftSaveDue) {
	FlowUtils{}.When(!due.TooLarge, func() {
		compose.html, compose.text, compose.quoteProof = due.HTML, due.Text, due.QuoteProof
		compose.clearQuoteWithoutProof()
	})()
	FlowUtils{}.Either(due.TooLarge, func() { compose.DraftStatus = ComposeLimitUtils{}.TooLarge() }, compose.autosave)()
	compose.publish(ComposeChanged{Reason: compose.names().Changed})
}

// SaveDraftCopy keeps the local edits as a new draft after a conflict.
func (compose *ComposeService) SaveDraftCopy() {
	compose.DraftID, compose.version, compose.saved, compose.DraftConflict = "", 0, "", false
	compose.clientID = compose.uuid()
	compose.saveSoon(compose.names().SaveNow)
}

// HearConfirmation replaces the local edits with the newer saved draft once
// the person confirmed it.
// @OnEvent
func (compose *ComposeService) HearConfirmation(given ConfirmationGiven) {
	FlowUtils{}.When(compose.reloadable(given.Action), compose.reloadDraft)()
}

// Discard deletes the server draft and leaves the composer.
func (compose *ComposeService) Discard() {
	compose.exiting = true
	compose.discardDraft(compose.DraftID)
	compose.leave(map[bool]string{true: "/scheduled", false: "/inbox"}[compose.ScheduledEditID != ""])
}

func (compose *ComposeService) publish(value any) {
	compose.applicationEventPublisher.PublishEvent(value, false)
}

func (compose *ComposeService) authorization() string {
	return "Bearer " + compose.session.Token
}

func (compose *ComposeService) toast(message string) {
	compose.publish(ToastRequested{Message: message})
}

func (compose *ComposeService) uuid() string {
	return IdentityUtils{}.UUID("compose|" + compose.session.Email + "|" + compose.DraftID)
}

func (ComposeService) names() ComposeNames {
	return ComposeNames{
		Changed: "changed", SaveNow: "save-now", SaveSoon: "save-soon", Start: "start", Send: "send", Archive: "archive",
		Schedule: "schedule", ScheduleEdit: "schedule-edit", Saved: "Saved", ReloadDraft: "compose-reload-draft",
		Draft: "draft", Recovered: "recovered", Open: "open", Prefill: "caduceus_compose_prefill", Pointer: "caduceus_compose_draft", Undo: "caduceus_pending_send_undo", Session: "session",
	}
}

func (compose *ComposeService) timeZone() string {
	zone, problem := compose.clock.TimeZone()
	return map[bool]string{true: zone, false: "local time"}[problem == nil && zone != ""]
}

// openWhenReady opens the composer once it knows the session, the launch and
// the tab's storage.
func (compose *ComposeService) openWhenReady() {
	ready := !slices.Contains([]bool{compose.session.SignedIn, compose.launched, compose.restored, !compose.opened}, false)
	FlowUtils{}.When(ready, compose.open)()
}

// open seeds the message from the prefill, the draft it names or the draft this
// tab was writing, like initFromPrefill in the former compose view.
func (compose *ComposeService) open() {
	compose.opened = true
	prefill := DraftUtils{}.Prefill(compose.stored.Session[compose.names().Prefill], compose.launch.Fields)
	pointer := DraftUtils{}.Pointer(compose.stored.Session[compose.names().Pointer])
	compose.useFields(DraftUtils{}.PrefillFields(prefill))
	compose.rememberQuote(prefill.Quote)
	compose.ScheduledEditID, compose.ScheduledSendAt = prefill.ScheduledID, prefill.SendAt
	opening := DraftUtils{}.Opening(prefill, pointer)
	map[string]func(){
		compose.names().Draft:     func() { compose.loadDraft(prefill.DraftID) },
		"prefill":                 compose.keepPrefill,
		compose.names().Recovered: func() { compose.recover(pointer) },
		"blank":                   func() {},
	}[opening]()
	compose.clientID = TextUtils{}.FirstOf(compose.clientID, compose.uuid())
	compose.showBody()
	compose.announceLoaded(compose.names().Open, opening == compose.names().Recovered)
}

// clearQuoteWithoutProof revokes the original prefill authority after a checked body loses it.
func (compose *ComposeService) clearQuoteWithoutProof() {
	if compose.quoteProof == nil {
		compose.EditorQuoteHTML = ""
	}
}

// rememberQuote accepts only the app-owned prefill record, never a marker in Body.
// Mailto launch fields have no quote record. Both strings are regenerated with escaping.
func (compose *ComposeService) rememberQuote(quote *QuotedMessage) {
	if quote != nil && slices.Contains([]string{"reply", "forward"}, quote.Mode) {
		compose.EditorQuoteHTML = QuotedMessageUtils{}.Html(*quote)
	}
}

func (compose *ComposeService) keepPrefill() {
	fields := compose.message()
	compose.firstSave = map[bool]string{true: compose.names().SaveSoon, false: compose.names().Changed}[DraftUtils{}.Meaningful(fields) && compose.ScheduledEditID == ""]
}

func (compose *ComposeService) loadDraft(id string) {
	draft, failure := compose.draftsClient.Draft(compose.authorization(), id)
	FlowUtils{}.Either(failure == nil, func() { compose.useDraft(draft) }, func() { compose.draftUnavailable(failure) })()
}

// recover reopens the draft this tab was writing before a reload. A draft
// that was sent or discarded meanwhile leaves a blank message.
func (compose *ComposeService) recover(pointer DraftPointer) {
	draft, failure := compose.draftsClient.Draft(compose.authorization(), pointer.DraftID)
	FlowUtils{}.Either(failure == nil, func() {
		compose.clientID = pointer.ClientID
		compose.useDraft(draft)
	}, func() { compose.publish(DraftUtils{}.Forgotten()) })()
}

func (compose *ComposeService) draftUnavailable(failure error) {
	compose.DraftStatus = "The saved draft could not be loaded."
	FlowUtils{}.When(HTTPUtils{}.Refused(failure), func() { compose.publish(SessionCheckRequested{Reason: "compose"}) })()
}

func (compose *ComposeService) useDraft(draft Draft) {
	compose.useFields(DraftUtils{}.Fields(draft))
	compose.DraftID, compose.version = draft.ID, draft.DraftVersion
	compose.clientID = TextUtils{}.FirstOf(draft.DraftClientID, compose.clientID)
	compose.saved = DraftUtils{}.Fingerprint(DraftUtils{}.Snapshot(compose.message(), compose.clientID, 0))
	compose.DraftStatus = map[bool]string{true: compose.names().Saved, false: ""}[draft.DraftVersion > 0]
}

func (compose *ComposeService) useFields(fields MessageFields) {
	compose.from = fields.From
	compose.To, compose.Cc, compose.Bcc, compose.ReplyTo, compose.Subject = fields.To, fields.Cc, fields.Bcc, fields.ReplyTo, fields.Subject
	compose.html, compose.text, compose.quoteProof = fields.HTML, fields.Text, fields.QuoteProof
	compose.inReplyTo, compose.references, compose.SourceEmailID = fields.InReplyTo, fields.References, fields.SourceEmailID
	compose.Attachments = compose.identified(fields.Attachments)
	compose.RequestReadReceipt, compose.TrackLinks, compose.RemindAt = fields.OpenTracking, fields.LinkTracking, fields.RemindAt
}

func (compose *ComposeService) identified(attachments []ComposeAttachment) []ComposeAttachment {
	kept := []ComposeAttachment{}
	for _, attachment := range attachments {
		attachment.ID = TextUtils{}.FirstOf(attachment.ID, compose.uuid())
		kept = append(kept, attachment)
	}
	return kept
}

func (compose *ComposeService) message() MessageFields {
	return MessageFields{
		From: compose.from, To: compose.To, Cc: compose.Cc, Bcc: compose.Bcc, ReplyTo: compose.ReplyTo, Subject: compose.Subject,
		HTML: compose.html, Text: compose.text, QuoteProof: compose.quoteProof, InReplyTo: compose.inReplyTo, References: compose.references, SourceEmailID: compose.SourceEmailID,
		Attachments: compose.Attachments, OpenTracking: compose.RequestReadReceipt, LinkTracking: compose.TrackLinks, RemindAt: compose.RemindAt,
	}
}

// showBody hands the editor a new document for the current body.
func (compose *ComposeService) showBody() {
	compose.generation++
	compose.EditorRevision = compose.generation
	compose.EditorBody = compose.html
	compose.EditorDocument = EditorUtils{}.Document(compose.html, compose.Attachments, compose.generation)
	compose.EditorSnapshot = ComposeEditorSnapshot{Document: compose.EditorDocument, Revision: compose.EditorRevision, Body: compose.EditorBody, Quote: compose.EditorQuoteHTML, Proof: compose.quoteProof, Attachments: append([]ComposeAttachment{}, compose.Attachments...)}
}

func (compose *ComposeService) announceLoaded(reason string, recovered bool) {
	compose.applicationEventPublisher.PublishEvent(ComposeLoaded{
		Reason: reason, From: compose.from, HTML: compose.html, DraftID: compose.DraftID, ScheduledID: compose.ScheduledEditID,
		Recovered: recovered, SignatureID: SignatureUtils{}.CurrentID(compose.html), Attachments: compose.Attachments,
	}, true)
}

func (compose *ComposeService) becomeReady() {
	compose.Ready = true
	compose.publish(ComposeChanged{Reason: "ready"})
	compose.saveSoon(compose.firstSave)
}

func (compose *ComposeService) saveSoon(reason string) {
	compose.publish(ComposeChanged{Reason: reason})
}

func (compose *ComposeService) mergeAutomatic(chosen SenderChosen) {
	FlowUtils{}.When(chosen.Apply, func() {
		compose.Cc = RecipientUtils{}.MergeAutomatic(compose.Cc, compose.automaticCc, chosen.NextCc)
		compose.Bcc = RecipientUtils{}.MergeAutomatic(compose.Bcc, compose.automaticBcc, chosen.NextBcc)
		compose.automaticCc, compose.automaticBcc = append([]string{}, chosen.NextCc...), append([]string{}, chosen.NextBcc...)
	})()
}

func (compose *ComposeService) recipientFields() map[string]*string {
	return map[string]*string{"to": &compose.To, "cc": &compose.Cc, "bcc": &compose.Bcc}
}

func (compose *ComposeService) setRecipient(field string, value string) {
	if target, found := compose.recipientFields()[field]; found {
		*target = value
	}
}

func (compose *ComposeService) suggestionsFor(query string) []ContactSuggestion {
	if query == "" {
		return nil
	}
	found, _ := compose.contactsClient.Suggestions(compose.authorization(), query)
	return RecipientUtils{}.Unused(found, compose.To, compose.Cc, compose.Bcc)
}

func (compose *ComposeService) showSuggestions(field string, found []ContactSuggestion) {
	compose.suggestions = found
	compose.RecipientField = field
	compose.RecipientIndex = min(0, len(found)-1)
	compose.renderSuggestions()
}

func (compose *ComposeService) hideSuggestions() {
	compose.recipientRequest++
	compose.suggestions = nil
	compose.RecipientIndex = -1
	compose.renderSuggestions()
	compose.RecipientField = ""
}

func (compose *ComposeService) renderSuggestions() {
	compose.RecipientOptions = RecipientUtils{}.Options(compose.suggestions, compose.RecipientField, compose.RecipientIndex)
	compose.publish(RecipientSuggestionsShown{Field: compose.RecipientField, Count: len(compose.suggestions), Active: compose.RecipientIndex})
}

func (compose *ComposeService) suggestionKeys(key string) func() {
	keys := map[string]func(){
		"ArrowDown": func() { compose.moveSuggestion(1) },
		"ArrowUp":   func() { compose.moveSuggestion(-1) },
		"Enter":     compose.chooseCurrent,
		"Escape":    compose.hideSuggestions,
	}
	if step, found := keys[key]; found {
		return step
	}
	return func() {}
}

func (compose *ComposeService) chooseCurrent() {
	compose.chooseSuggestion(compose.suggestionAt(compose.RecipientIndex))
}

func (compose *ComposeService) listed(field string) bool {
	return field == compose.RecipientField && len(compose.suggestions) > 0
}

func (compose *ComposeService) starting(reason string) bool {
	return reason == compose.names().Start && !compose.Ready
}

func (compose *ComposeService) reloadable(action string) bool {
	return action == compose.names().ReloadDraft && compose.DraftID != ""
}

func (compose *ComposeService) defaultMode() string {
	if compose.preferences.DefaultSendAction == "send-and-archive" && compose.SourceEmailID != "" {
		return compose.names().Archive
	}
	return compose.names().Send
}

func (compose *ComposeService) command(command string) func() {
	commands := map[string]func(){"composer:send": compose.SendDefault, "composer:discard": compose.Discard}
	if step, found := commands[command]; found {
		return step
	}
	return func() {}
}

func (compose *ComposeService) moveSuggestion(direction int) {
	count := len(compose.suggestions)
	compose.RecipientIndex = (compose.RecipientIndex + direction + count) % count
	compose.renderSuggestions()
}

func (compose *ComposeService) suggestionAt(index int) []ContactSuggestion {
	if index < 0 || index >= len(compose.suggestions) || compose.RecipientField == "" {
		return nil
	}
	return compose.suggestions[index : index+1]
}

func (compose *ComposeService) chooseSuggestion(chosen []ContactSuggestion) {
	for _, suggestion := range chosen {
		compose.useSuggestion(compose.RecipientField, suggestion)
	}
}

func (compose *ComposeService) useSuggestion(field string, suggestion ContactSuggestion) {
	value := RecipientUtils{}.ReplaceCurrent(*compose.recipientFields()[field], suggestion.Email)
	compose.setRecipient(field, value)
	compose.hideSuggestions()
	compose.publish(RecipientChosen{Field: field, Value: value})
	compose.saveSoon(compose.names().SaveSoon)
}

func (compose *ComposeService) attachedBytes() int {
	total := 0
	for _, attachment := range compose.Attachments {
		total += attachment.Size
	}
	return total
}

func (compose *ComposeService) acceptAll(files []AttachmentOffer) []AttachmentAcceptance {
	total := compose.attachedBytes()
	accepted := []AttachmentAcceptance{}
	for _, file := range files {
		accepted = append(accepted, compose.acceptFile(file, ComposeLimitUtils{}.Problem(file, len(compose.Attachments)+len(accepted), total), &total)...)
	}
	return accepted
}

func (compose *ComposeService) acceptFile(file AttachmentOffer, problem string, total *int) []AttachmentAcceptance {
	accepted := []AttachmentAcceptance{}
	FlowUtils{}.Either(problem == "", func() {
		*total += file.Size
		id := compose.uuid()
		accepted = append(accepted, AttachmentAcceptance{Index: file.Index, ID: id, ContentID: EditorUtils{}.InlineContentID(file.Inline, id)})
	}, func() { compose.toast(problem) })()
	return accepted
}

func (compose *ComposeService) attachmentsWhere(id string, named bool) []ComposeAttachment {
	kept := []ComposeAttachment{}
	for _, attachment := range compose.Attachments {
		kept = append(kept, map[bool][]ComposeAttachment{true: {attachment}, false: nil}[(attachment.ID == id) == named]...)
	}
	return kept
}

func (compose *ComposeService) isFuture(at string) bool {
	moment, problem := time.Parse(time.RFC3339Nano, at)
	now, clockProblem := compose.clock.NowMs()
	return !slices.Contains([]bool{problem == nil, clockProblem == nil, float64(moment.UnixMilli()) > now}, false)
}

func (compose *ComposeService) problemFor(mode string) string {
	return SendUtils{}.Problem(compose.message(), mode)
}

func (compose *ComposeService) accepted(problem string) bool {
	compose.Error = problem
	return problem == ""
}

func (compose *ComposeService) sendAs(mode string) {
	compose.Error = compose.problemFor(mode)
	FlowUtils{}.When(!slices.Contains([]bool{compose.Error == "", !compose.Loading}, false), func() { compose.requestSend(SendChoice{Mode: mode}) })()
}

// requestSend asks the editor for the body; HearSendBody sends it.
func (compose *ComposeService) requestSend(choice SendChoice) {
	choice.DelayMs, choice.RichText = compose.preferences.UndoSendDelay, compose.preferences.RichText
	choice.IdempotencyKey = "send:" + compose.uuid()
	compose.choice = choice
	compose.Loading, compose.Error = true, ""
	compose.publish(SendRequested{Mode: choice.Mode})
}

func (compose *ComposeService) deliver(ready SendBodyReady) {
	fields := compose.message()
	fields.HTML, fields.Text, fields.QuoteProof = ready.HTML, ready.Text, ready.QuoteProof
	input := SendUtils{}.Input(fields, compose.choice)
	problem := ComposeLimitUtils{}.RequestProblem(input)
	FlowUtils{}.Either(problem == "", func() { compose.post(input) }, func() { compose.Loading, compose.Error = false, problem })()
}

func (compose *ComposeService) post(input SendInput) {
	FlowUtils{}.When(SendUtils{}.Sounded(compose.choice.Mode), func() { compose.publish(SoundRequested{Sound: "hit-send"}) })()
	answer, failure := compose.sent(input)
	FlowUtils{}.Either(failure == nil, func() { compose.finishSend(answer) }, func() {
		compose.Loading, compose.Error = false, SendUtils{}.Failure(compose.choice.Mode, failure)
	})()
}

func (compose *ComposeService) sent(input SendInput) (SendAnswer, error) {
	if compose.choice.Mode == compose.names().ScheduleEdit {
		return compose.sendClient.Reschedule(compose.authorization(), compose.ScheduledEditID, input)
	}
	return compose.sendClient.Send(compose.authorization(), input)
}

// finishSend leaves the composer after a send or a schedule: the draft is
// consumed, the undo offer goes to the shell through sessionStorage, and the
// inbox or the scheduled list opens.
func (compose *ComposeService) finishSend(answer SendAnswer) {
	compose.exiting = true
	FlowUtils{}.When(SendUtils{}.Sounded(compose.choice.Mode), func() { compose.publish(SoundRequested{Sound: compose.names().Send}) })()
	FlowUtils{}.When(compose.ScheduledEditID == "", func() { compose.discardDraft(compose.DraftID) })()
	FlowUtils{}.When(answer.UndoID != "", func() { compose.offerUndo(answer) })()
	compose.leave(SendUtils{}.Route(compose.choice.Mode))
}

func (compose *ComposeService) offerUndo(answer SendAnswer) {
	now, _ := compose.clock.NowMs()
	pending := PendingUndo{UndoID: answer.UndoID, Owner: compose.session.Email, Message: SendUtils{}.Sent(compose.choice.Mode, answer), ExpiresAt: int64(now) + int64(compose.choice.DelayMs)}
	encoded, _ := json.Marshal(pending)
	compose.publish(StorageWritten{Area: compose.names().Session, Values: map[string]string{compose.names().Undo: string(encoded)}, Removed: []string{}})
}

func (compose *ComposeService) discardDraft(id string) {
	FlowUtils{}.When(id != "", func() { compose.draftsClient.Discard(compose.authorization(), id) })()
	compose.DraftID = ""
}

func (compose *ComposeService) leave(target string) {
	compose.exiting = true
	compose.publish(DraftUtils{}.Forgotten())
	compose.publish(NavigationRequested{Target: target})
}

// autosave saves one complete versioned snapshot when it changed.
func (compose *ComposeService) autosave() {
	snapshot := DraftUtils{}.Snapshot(compose.message(), compose.clientID, compose.version)
	allowed := !slices.Contains([]bool{compose.Ready, !compose.Loading, !compose.exiting, compose.ScheduledEditID == "", !compose.DraftConflict}, false)
	FlowUtils{}.When(allowed, compose.saveStep(snapshot))()
}

func (compose *ComposeService) saveStep(snapshot DraftInput) func() {
	switch {
	case ComposeLimitUtils{}.DraftProblem(snapshot) != "":
		return func() { compose.DraftStatus = ComposeLimitUtils{}.TooLarge() }
	case !DraftUtils{}.Worth(snapshot, compose.saved, compose.DraftID):
		return func() {
			compose.DraftStatus = map[bool]string{true: compose.names().Saved, false: compose.DraftStatus}[compose.DraftID != ""]
		}
	}
	return func() { compose.saveDraft(snapshot) }
}

func (compose *ComposeService) saveDraft(snapshot DraftInput) {
	draft, failure := compose.savedDraft(snapshot)
	FlowUtils{}.Either(failure == nil, func() { compose.afterSave(draft, DraftUtils{}.Fingerprint(snapshot)) }, func() {
		compose.DraftStatus = HTTPUtils{}.Message(failure, "Autosave failed", "Not saved: check your connection. A reload keeps only the last saved version.")
	})()
}

func (compose *ComposeService) savedDraft(snapshot DraftInput) (Draft, error) {
	if compose.DraftID == "" {
		return compose.draftsClient.Create(compose.authorization(), snapshot)
	}
	return compose.draftsClient.Update(compose.authorization(), compose.DraftID, snapshot)
}

func (compose *ComposeService) afterSave(draft Draft, fingerprint string) {
	FlowUtils{}.Either(draft.Conflict, func() {
		compose.DraftConflict, compose.DraftStatus = true, "Newer saved version detected"
	}, func() { compose.keepSaved(draft, fingerprint) })()
}

func (compose *ComposeService) keepSaved(draft Draft, fingerprint string) {
	compose.DraftID, compose.version, compose.saved = draft.ID, max(draft.DraftVersion, 1), fingerprint
	compose.clientID = TextUtils{}.FirstOf(draft.DraftClientID, compose.clientID)
	compose.DraftStatus, compose.DraftConflict = compose.names().Saved, false
	compose.publish(DraftUtils{}.Stored(DraftPointer{DraftID: compose.DraftID, ClientID: compose.clientID, Version: compose.version}))
	compose.publish(InboxRefreshRequested{Reason: compose.names().Draft})
}

func (compose *ComposeService) reloadDraft() {
	compose.loadDraft(compose.DraftID)
	compose.DraftConflict, compose.DraftStatus = false, "Saved version loaded"
	compose.showBody()
	compose.announceLoaded("reload", false)
}
