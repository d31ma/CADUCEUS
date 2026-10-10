import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ComposeService holds the header fields, recipient suggestions, reminders,
// scheduling, tracking choices and attachment list of the compose view, and
// it builds the send envelope. SenderService owns the From identity and the
// signatures, and TemplateService the templates. The message body and the
// attachment bytes stay in ComposeEditorService (JavaScript): a Go call, an
// event and a browser HTTP request each carry at most 64 KiB, and a message
// with attachments carries up to 25 MB. The services talk through the event
// models in client/models.
//
// @Service
type ComposeService struct {
	// @Needs
	contactsClient *ContactsClient
	// @Needs
	draftsClient *DraftsClient
	// @Needs
	clock Clock
	// @Needs
	secureRandom              SecureRandom
	applicationEventPublisher ApplicationEventPublisher

	To                 string
	Cc                 string
	BlindCopy          string
	ReplyTo            string
	Subject            string
	Loading            bool
	Ready              bool
	Error              string
	DraftID            string
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

	token            string
	from             string
	inReplyTo        string
	references       []string
	unsavedDelay     int
	intentionalExit  bool
	automaticCc      []string
	automaticBlind   []string
	preferences      MailPreferences
	suggestions      []RecipientSuggestion
	recipientRequest int
}

// @OnStart
func (compose *ComposeService) Start() {
	compose.preferences = MailPreferencesUtils{}.Defaults()
	compose.Attachments = []ComposeAttachment{}
	compose.RecipientOptions = []RecipientOption{}
	compose.RecipientIndex = -1
	compose.unsavedDelay = -1
	compose.ScheduleTimeZone = compose.timeZone()
}

// SessionChanged keeps the signed-in mailbox that every request names.
//
// @OnEvent({"replay":true})
func (compose *ComposeService) SessionChanged(changed SessionChanged) {
	compose.token = changed.Token
}

// Loaded seeds the header fields from the prefill, the recovered draft or a
// reloaded draft, like initFromPrefill in the former compose view.
//
// @OnEvent({"replay":true})
func (compose *ComposeService) Loaded(loaded ComposeLoaded) {
	compose.To, compose.Cc, compose.BlindCopy, compose.ReplyTo = loaded.To, loaded.Cc, loaded.BlindCopy, loaded.ReplyTo
	compose.Subject, compose.inReplyTo = loaded.Subject, loaded.InReplyTo
	compose.references = append([]string{}, loaded.References...)
	compose.ScheduledEditID, compose.ScheduledSendAt = loaded.ScheduledID, loaded.SendAt
	compose.SourceEmailID, compose.RemindAt = loaded.SourceEmailID, loaded.RemindAt
	compose.RequestReadReceipt, compose.TrackLinks = loaded.OpenTracking, loaded.LinkTracking
	compose.DraftID = loaded.DraftID
	compose.Attachments = append([]ComposeAttachment{}, loaded.Attachments...)
	compose.unsavedDelay = compose.firstSaveDelay(loaded)
}

// SenderChosen follows the From identity that SenderService chose, merges
// its automatic CC and BCC recipients and, on start, marks the view ready.
//
// @OnEvent({"replay":true})
func (compose *ComposeService) SenderChosen(chosen SenderChosen) {
	compose.from = chosen.Address
	compose.preferences = chosen.Preferences
	compose.mergeAutomatic(chosen)
	compose.becomeReady(chosen.Reason, compose.saveReason(compose.unsavedDelay))
}

// TypeRecipient stores a To, CC or BCC value as the person types.
func (compose *ComposeService) TypeRecipient(field string, value string) {
	compose.setRecipient(field, value)
}

// RecipientQueryChanged loads suggestions for the address being typed. The
// editor service debounces typing for 140 ms before it publishes the query.
//
// @OnEvent
func (compose *ComposeService) RecipientQueryChanged(changed RecipientQueryChanged) {
	compose.recipientRequest++
	request := compose.recipientRequest
	found := compose.suggestionsFor(RecipientUtils{}.CurrentToken(changed.Value))
	compose.showSuggestions(changed.Field, found, request)
	compose.publish(ComposeChanged{Reason: compose.changedReason()})
}

// RecipientSuggestionsDismissed hides the list when focus leaves the fields.
//
// @OnEvent
func (compose *ComposeService) RecipientSuggestionsDismissed(dismissed RecipientSuggestionsDismissed) {
	compose.hideSuggestions()
}

// RecipientKey moves through, picks or closes the suggestion list.
func (compose *ComposeService) RecipientKey(field string, key string) {
	compose.navigateSuggestions(compose.suggestionsOpenFor(field), key)
}

// ChooseRecipient replaces the address being typed with a suggestion.
func (compose *ComposeService) ChooseRecipient(index int) {
	compose.chooseSuggestion(compose.suggestionAt(index))
}

// ChangeTracking records the open or link tracking choice.
func (compose *ComposeService) ChangeTracking(option string, enabled bool) {
	compose.setTracking(option, enabled)
	compose.saveSoon(0)
}

// TemplateInserted copies the template's recipients and subject once the
// editor has inserted its body.
//
// @OnEvent
func (compose *ComposeService) TemplateInserted(inserted TemplateInserted) {
	compose.To = compose.firstOf(inserted.To, compose.To)
	compose.Cc = compose.firstOf(inserted.Cc, compose.Cc)
	compose.Subject = compose.firstOf(inserted.Subject, compose.Subject)
	compose.saveSoon(0)
}

// AttachmentsOffered applies the attachment limits (10 files, 10 MB each,
// 25 MB in total) before the editor reads any file.
//
// @OnEvent
func (compose *ComposeService) AttachmentsOffered(offered AttachmentsOffered) {
	total := compose.attachedBytes()
	accepted := compose.acceptFiles(offered.Files, &total)
	compose.publish(AttachmentsAccepted{Accepted: accepted})
}

// AttachmentsAdded lists the files the editor has read.
//
// @OnEvent
func (compose *ComposeService) AttachmentsAdded(added AttachmentsAdded) {
	compose.Attachments = append(append([]ComposeAttachment{}, compose.Attachments...), added.Attachments...)
	compose.saveSoon(0)
}

// RemoveAttachment drops a pending attachment and its inline image.
func (compose *ComposeService) RemoveAttachment(id string) {
	removed := compose.attachmentNamed(id)
	compose.Attachments = compose.attachmentsWithout(id)
	compose.publish(AttachmentRemoved{ID: id, ContentID: removed.ContentID})
	compose.saveSoon(0)
}

// InlineImageRemoved drops the attachment of an inline image the person
// removed in the editor.
//
// @OnEvent
func (compose *ComposeService) InlineImageRemoved(removed InlineImageRemoved) {
	compose.Attachments = compose.attachmentsWithout(removed.ID)
	compose.saveSoon(0)
}

// AttachmentSize writes the size shown on an attachment pill.
func (compose *ComposeService) AttachmentSize(size int) string {
	return AttachmentUtils{}.SizeLabel(size)
}

// ChooseFollowUpReminder opens or closes the follow-up reminder picker.
func (compose *ComposeService) ChooseFollowUpReminder() {
	compose.ReminderPickerOpen = !compose.ReminderPickerOpen
}

// SetFollowUpReminder keeps a future reminder time chosen in the picker.
func (compose *ComposeService) SetFollowUpReminder(at string) {
	compose.keepReminder(at, compose.isFuture(at))
}

// CloseFollowUpReminder closes the reminder picker.
func (compose *ComposeService) CloseFollowUpReminder() {
	compose.ReminderPickerOpen = false
}

// ClearFollowUpReminder removes the reminder.
func (compose *ComposeService) ClearFollowUpReminder() {
	compose.RemindAt = ""
	compose.ReminderPickerOpen = false
	compose.saveSoon(0)
}

// ChooseSendLater opens the send-later picker once To and Subject are set.
func (compose *ComposeService) ChooseSendLater() {
	compose.toggleSchedulePicker(compose.accepted(compose.requiredProblem()))
}

// CloseSendLater closes the send-later picker.
func (compose *ComposeService) CloseSendLater() {
	compose.SchedulePickerOpen = false
}

// ScheduleAt schedules the message, or moves an existing schedule, to a
// future time.
func (compose *ComposeService) ScheduleAt(at string) {
	request := SendRequested{Mode: compose.scheduleKind(), SendAt: at, ScheduledID: compose.ScheduledEditID}
	compose.requestSchedule(request, compose.isFuture(at))
}

// Send sends now; the server holds the message for the undo-send delay.
func (compose *ComposeService) Send() {
	compose.requestSend(compose.sendMode(), compose.accepted(compose.requiredProblem()))
}

// SendAndArchive sends and archives the source conversation.
func (compose *ComposeService) SendAndArchive() {
	compose.requestSend(compose.archiveMode(), compose.accepted(compose.problemFor(compose.archiveMode())))
}

// SendDefault follows the default send action of the mail preferences.
func (compose *ComposeService) SendDefault() {
	mode := compose.defaultMode()
	compose.requestSend(mode, compose.accepted(compose.problemFor(mode)))
}

// ComposeShortcut runs Ctrl+Enter and the composer shortcuts.
//
// @OnEvent
func (compose *ComposeService) ComposeShortcut(shortcut ComposeShortcut) {
	compose.runShortcut(shortcut.Action)
	compose.publish(ComposeChanged{Reason: compose.changedReason()})
}

// SendFinished clears the form after a send, a queued send or a schedule,
// or shows why the editor could not send.
//
// @OnEvent
func (compose *ComposeService) SendFinished(finished SendFinished) {
	compose.Loading = false
	compose.finishSend(finished, compose.ScheduledEditID != "")
	compose.publish(ComposeChanged{Reason: compose.changedReason()})
}

// DraftSaveDue builds the header part of an autosave once the editor's
// debounce timer fires.
//
// @OnEvent
func (compose *ComposeService) DraftSaveDue(due DraftSaveDue) {
	compose.requestDraftSave(compose.mayAutosave())
}

// DraftSaveFinished follows the draft identity the server acknowledged.
//
// @OnEvent
func (compose *ComposeService) DraftSaveFinished(finished DraftSaveFinished) {
	compose.DraftID = finished.DraftID
}

// Discard deletes the server draft and leaves the composer.
func (compose *ComposeService) Discard() {
	compose.intentionalExit = true
	route := compose.exitRoute()
	discarded := compose.discardDraft(compose.DraftID)
	compose.publish(NavigationRequested{Route: route, ClearRecovery: true, DiscardDraft: discarded})
}

func (compose *ComposeService) publish(value any) {
	compose.applicationEventPublisher.PublishEvent(value, false)
}

func (compose *ComposeService) authorization() string {
	return "Bearer " + compose.token
}

func (compose *ComposeService) toast(message string) {
	compose.publish(ToastRequested{Message: message})
}

func (ComposeService) changedReason() string { return "changed" }

func (ComposeService) sendMode() string { return "send" }

func (ComposeService) archiveMode() string { return "archive" }

func (ComposeService) scheduledStatus() string { return "scheduled" }

func (ComposeService) queuedStatus() string { return "queued" }

func (ComposeService) sentStatus() string { return "sent" }

func (ComposeService) scheduleMode() string { return "schedule" }

func (ComposeService) saveNowReason() string { return "save-now" }

func (ComposeService) inboxRoute() string { return "inbox" }

func (ComposeService) requiredMessage() string { return "To and Subject are required." }

func (ComposeService) toField() string { return "to" }

func (ComposeService) ccField() string { return "cc" }

func (ComposeService) networkFailure() string { return "HTTP_NETWORK" }

func (ComposeService) typingDelay() int { return 750 }

func (ComposeService) mostAttachments() int { return 10 }

func (ComposeService) kilobyte() int { return 1024 }

func (compose *ComposeService) megabyte() int { return compose.kilobyte() * compose.kilobyte() }

func (compose *ComposeService) mostFileBytes() int { return 10 * compose.megabyte() }

func (compose *ComposeService) mostTotalBytes() int { return 25 * compose.megabyte() }

func (ComposeService) inlineImageTypes() []string {
	return []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/avif"}
}

func (compose *ComposeService) timeZone() string {
	zone, problem := compose.clock.TimeZone()
	return compose.orLocalTime(zone, problem)
}

func (ComposeService) orLocalTime(zone string, problem error) string {
	if problem != nil || zone == "" {
		return "local time"
	}
	return zone
}

func (ComposeService) firstOf(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// firstSaveDelay asks for the first durable save of a new prefill or a
// local-only recovery; a server-backed draft already has its fingerprint.
func (compose *ComposeService) firstSaveDelay(loaded ComposeLoaded) int {
	if loaded.ScheduledID == "" && loaded.Meaningful && !loaded.SavedSnapshot {
		return compose.typingDelay()
	}
	return -1
}

func (compose *ComposeService) mergeAutomatic(chosen SenderChosen) {
	compose.Cc = compose.merged(chosen.Apply, compose.Cc, compose.automaticCc, chosen.NextCc)
	compose.BlindCopy = compose.merged(chosen.Apply, compose.BlindCopy, compose.automaticBlind, chosen.NextBlindCopy)
	compose.automaticCc = compose.kept(chosen.Apply, compose.automaticCc, chosen.NextCc)
	compose.automaticBlind = compose.kept(chosen.Apply, compose.automaticBlind, chosen.NextBlindCopy)
}

func (ComposeService) merged(apply bool, current string, previous []string, next []string) string {
	if apply {
		return RecipientUtils{}.MergeAutomatic(current, previous, next)
	}
	return current
}

func (ComposeService) kept(apply bool, previous []string, next []string) []string {
	if apply {
		return append([]string{}, next...)
	}
	return previous
}

func (compose *ComposeService) becomeReady(reason string, firstSave string) {
	if reason != "start" || compose.Ready {
		compose.publish(ComposeChanged{Reason: compose.saveNowReason()})
		return
	}
	compose.Ready = true
	compose.publish(ComposeChanged{Reason: "ready"})
	compose.publish(ComposeChanged{Reason: firstSave})
}

func (compose *ComposeService) saveSoon(delay int) {
	compose.publish(ComposeChanged{Reason: compose.saveReason(delay)})
}

func (compose *ComposeService) saveReason(delay int) string {
	if delay < 0 {
		return compose.changedReason()
	} else if delay == 0 {
		return compose.saveNowReason()
	}
	return "save-soon"
}

func (compose *ComposeService) setRecipient(field string, value string) {
	switch field {
	case compose.toField():
		compose.To = value
	case compose.ccField():
		compose.Cc = value
	default:
		compose.BlindCopy = value
	}
}

func (compose *ComposeService) recipientValue(field string) string {
	switch field {
	case compose.toField():
		return compose.To
	case compose.ccField():
		return compose.Cc
	}
	return compose.BlindCopy
}

func (compose *ComposeService) suggestionsFor(query string) []RecipientSuggestion {
	if query == "" {
		return nil
	}
	found, _ := compose.contactsClient.Suggestions(query, compose.authorization())
	return RecipientUtils{}.Unused(found, compose.To, compose.Cc, compose.BlindCopy)
}

func (compose *ComposeService) showSuggestions(field string, found []RecipientSuggestion, request int) {
	if request != compose.recipientRequest {
		return
	}
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
	compose.RecipientOptions = compose.recipientOptions()
	compose.publish(RecipientSuggestionsShown{Field: compose.RecipientField, Count: len(compose.suggestions), Active: compose.RecipientIndex})
}

func (compose *ComposeService) recipientOptions() []RecipientOption {
	options := []RecipientOption{}
	for index, suggestion := range compose.suggestions {
		options = append(options, RecipientOption{
			Index:    index,
			OptionID: "compose-" + compose.RecipientField + "-suggestions-" + strconv.Itoa(index),
			Label:    compose.firstOf(suggestion.Name, suggestion.Email),
			Detail:   compose.suggestionDetail(suggestion),
			Initials: RecipientUtils{}.Initials(suggestion.Name, suggestion.Email),
			Active:   index == compose.RecipientIndex,
		})
	}
	return options
}

func (ComposeService) suggestionDetail(suggestion RecipientSuggestion) string {
	if suggestion.Name != "" {
		return suggestion.Email
	} else if suggestion.Source == "mail" {
		return "Found in mail"
	}
	return "Contact"
}

func (compose *ComposeService) suggestionsOpenFor(field string) bool {
	if field == compose.RecipientField && len(compose.suggestions) > 0 {
		return true
	}
	return false
}

func (compose *ComposeService) navigateSuggestions(listed bool, key string) {
	switch {
	case !listed:
		return
	case key == "ArrowDown":
		compose.moveSuggestion(1)
	case key == "ArrowUp":
		compose.moveSuggestion(-1)
	case key == "Enter":
		compose.chooseCurrent()
	case key == "Escape":
		compose.hideSuggestions()
	}
}

func (compose *ComposeService) chooseCurrent() {
	compose.chooseSuggestion(compose.suggestionAt(compose.RecipientIndex))
}

func (compose *ComposeService) moveSuggestion(direction int) {
	count := len(compose.suggestions)
	compose.RecipientIndex = (compose.RecipientIndex + direction + count) % count
	compose.renderSuggestions()
}

func (compose *ComposeService) suggestionAt(index int) []RecipientSuggestion {
	if index < 0 || index >= len(compose.suggestions) || compose.RecipientField == "" {
		return nil
	}
	return compose.suggestions[index : index+1]
}

func (compose *ComposeService) chooseSuggestion(chosen []RecipientSuggestion) {
	for _, suggestion := range chosen {
		compose.useSuggestion(compose.RecipientField, suggestion)
	}
}

func (compose *ComposeService) useSuggestion(field string, suggestion RecipientSuggestion) {
	value := RecipientUtils{}.ReplaceCurrent(compose.recipientValue(field), suggestion.Email)
	compose.setRecipient(field, value)
	compose.hideSuggestions()
	compose.publish(RecipientChosen{Field: field, Value: value})
	compose.saveSoon(compose.typingDelay())
}

func (compose *ComposeService) setTracking(option string, enabled bool) {
	switch option {
	case "open":
		compose.RequestReadReceipt = enabled
	case "links":
		compose.TrackLinks = enabled
	}
}

func (compose *ComposeService) attachedBytes() int {
	total := 0
	for _, attachment := range compose.Attachments {
		total += attachment.Size
	}
	return total
}

func (compose *ComposeService) acceptFiles(files []AttachmentOffer, total *int) []AttachmentAcceptance {
	accepted := []AttachmentAcceptance{}
	for _, file := range files {
		problem := compose.fileProblem(file, len(compose.Attachments)+len(accepted), *total)
		accepted = append(accepted, compose.acceptFile(file, problem, total)...)
	}
	return accepted
}

func (compose *ComposeService) acceptFile(file AttachmentOffer, problem string, total *int) []AttachmentAcceptance {
	if problem != "" {
		compose.toast(problem)
		return nil
	}
	*total += file.Size
	id, _ := compose.secureRandom.UUID()
	return []AttachmentAcceptance{{Index: file.Index, ID: id, ContentID: AttachmentUtils{}.InlineContentID(file.Inline, id)}}
}

func (compose *ComposeService) fileProblem(file AttachmentOffer, count int, total int) string {
	if count >= compose.mostAttachments() {
		return "You can attach up to 10 files."
	} else if file.Size > compose.mostFileBytes() {
		return file.Filename + " is larger than 10MB."
	} else if total+file.Size > compose.mostTotalBytes() {
		return "Attachments cannot exceed 25MB total."
	} else if file.Inline && !slices.Contains(compose.inlineImageTypes(), strings.ToLower(file.ContentType)) {
		return file.Filename + " is not a supported inline image."
	}
	return ""
}

func (compose *ComposeService) attachmentNamed(id string) ComposeAttachment {
	found := ComposeAttachment{}
	for _, attachment := range compose.Attachments {
		found = compose.namedOr(attachment, id, found)
	}
	return found
}

func (ComposeService) namedOr(attachment ComposeAttachment, id string, found ComposeAttachment) ComposeAttachment {
	if attachment.ID == id {
		return attachment
	}
	return found
}

func (compose *ComposeService) attachmentsWithout(id string) []ComposeAttachment {
	kept := []ComposeAttachment{}
	for _, attachment := range compose.Attachments {
		kept = append(kept, compose.unlessNamed(attachment, id)...)
	}
	return kept
}

func (ComposeService) unlessNamed(attachment ComposeAttachment, id string) []ComposeAttachment {
	if attachment.ID == id {
		return nil
	}
	return []ComposeAttachment{attachment}
}

func (compose *ComposeService) isFuture(at string) bool {
	moment, problem := time.Parse(time.RFC3339Nano, at)
	now, clockProblem := compose.clock.NowMs()
	return compose.after(moment, now, []error{problem, clockProblem})
}

func (ComposeService) after(moment time.Time, now float64, problems []error) bool {
	if problems[0] == nil && problems[1] == nil && float64(moment.UnixMilli()) > now {
		return true
	}
	return false
}

func (compose *ComposeService) keepReminder(at string, future bool) {
	if !future {
		compose.toast("Please choose a future reminder time.")
		return
	}
	compose.RemindAt = at
	compose.ReminderPickerOpen = false
	compose.saveSoon(0)
}

func (compose *ComposeService) requiredProblem() string {
	if strings.TrimSpace(compose.To) == "" || strings.TrimSpace(compose.Subject) == "" {
		return compose.requiredMessage()
	}
	return ""
}

func (compose *ComposeService) problemFor(mode string) string {
	if strings.TrimSpace(compose.To) == "" || strings.TrimSpace(compose.Subject) == "" {
		return compose.requiredMessage()
	} else if mode == compose.archiveMode() && compose.SourceEmailID == "" {
		return "Send & Archive is available only from a source conversation."
	}
	return ""
}

func (compose *ComposeService) defaultMode() string {
	if compose.preferences.DefaultSendAction == "send-and-archive" && compose.SourceEmailID != "" {
		return compose.archiveMode()
	}
	return compose.sendMode()
}

func (compose *ComposeService) accepted(problem string) bool {
	compose.Error = problem
	return problem == ""
}

func (compose *ComposeService) toggleSchedulePicker(valid bool) {
	if valid {
		compose.SchedulePickerOpen = !compose.SchedulePickerOpen
		compose.ReminderPickerOpen = false
	}
}

func (compose *ComposeService) requestSend(mode string, valid bool) {
	if valid {
		compose.startRequest(SendRequested{Mode: mode, DelayMs: int(compose.preferences.UndoSendDelay)})
	}
}

func (compose *ComposeService) requestSchedule(request SendRequested, future bool) {
	if !future {
		compose.toast("Please choose a future time.")
		return
	}
	compose.startRequest(request)
}

func (compose *ComposeService) scheduleKind() string {
	if compose.ScheduledEditID != "" {
		return "schedule-edit"
	}
	return compose.scheduleMode()
}

func (compose *ComposeService) startRequest(request SendRequested) {
	key, _ := compose.secureRandom.UUID()
	request.Envelope = compose.envelope()
	request.DraftID = compose.DraftID
	request.IdempotencyKey = "send:" + key
	compose.Loading = true
	compose.Error = ""
	compose.publish(request)
}

func (compose *ComposeService) envelope() MessageEnvelope {
	return MessageEnvelope{
		From:          compose.from,
		To:            RecipientUtils{}.Split(compose.To),
		Cc:            RecipientUtils{}.Split(compose.Cc),
		BlindCopy:     RecipientUtils{}.Split(compose.BlindCopy),
		ReplyTo:       RecipientUtils{}.Split(compose.ReplyTo),
		Subject:       compose.Subject,
		InReplyTo:     compose.inReplyTo,
		References:    append([]string{}, compose.references...),
		SourceEmailID: compose.SourceEmailID,
		OpenTracking:  compose.RequestReadReceipt,
		LinkTracking:  compose.TrackLinks,
		RemindAt:      compose.RemindAt,
	}
}

func (compose *ComposeService) runShortcut(action string) {
	switch action {
	case compose.sendMode():
		compose.SendDefault()
	case "discard":
		compose.Discard()
	}
}

func (compose *ComposeService) finishSend(finished SendFinished, editing bool) {
	switch finished.Status {
	case compose.sentStatus(), compose.queuedStatus(), compose.scheduledStatus():
		compose.leaveAfterSend(compose.sentNavigation(finished), editing)
	default:
		compose.Error = finished.Message
	}
}

func (compose *ComposeService) leaveAfterSend(navigation NavigationRequested, editing bool) {
	compose.intentionalExit = true
	navigation.DiscardDraft = compose.discardDraft(compose.draftToConsume(editing))
	compose.clearMessage()
	compose.publish(navigation)
}

func (compose *ComposeService) sentNavigation(finished SendFinished) NavigationRequested {
	return NavigationRequested{
		Route:         compose.routeAfter(finished),
		ClearRecovery: true,
		UndoID:        finished.UndoID,
		UndoLabel:     compose.undoLabel(finished),
		UndoDelay:     int(compose.preferences.UndoSendDelay),
		Toast:         compose.scheduledToast(finished.Status),
		ToastDate:     finished.SendAt,
	}
}

func (compose *ComposeService) routeAfter(finished SendFinished) string {
	if finished.Status == compose.scheduledStatus() || (finished.Status == compose.queuedStatus() && strings.HasPrefix(finished.Mode, compose.scheduleMode())) {
		return compose.scheduledStatus()
	}
	return compose.inboxRoute()
}

func (compose *ComposeService) scheduledToast(status string) string {
	switch {
	case status != compose.scheduledStatus():
		return ""
	case compose.ScheduledEditID != "":
		return "Schedule updated for "
	}
	return "Scheduled for "
}

func (compose *ComposeService) undoLabel(finished SendFinished) string {
	switch {
	case finished.Status != compose.sentStatus():
		return ""
	case finished.Mode != compose.archiveMode():
		return "Message sent. Undo"
	case finished.Archived:
		return "Sent & archived. Undo"
	}
	return "Message sent; source was not archived. Undo"
}

func (compose *ComposeService) draftToConsume(editing bool) string {
	if editing {
		return ""
	}
	return compose.DraftID
}

// discardDraft deletes the server draft. It answers the draft id when the
// network failed, so the editor queues the delete in the offline vault.
func (compose *ComposeService) discardDraft(id string) string {
	queued := compose.queuedDelete(id, compose.deleteDraft(id))
	compose.DraftID = ""
	return queued
}

func (compose *ComposeService) deleteDraft(id string) string {
	if id == "" {
		return ""
	}
	_, problem := compose.draftsClient.Discard(id, compose.authorization())
	return BehaviorErrorCode(problem)
}

func (compose *ComposeService) queuedDelete(id string, code string) string {
	if code == compose.networkFailure() {
		return id
	}
	return ""
}

func (compose *ComposeService) clearMessage() {
	compose.To, compose.Cc, compose.BlindCopy, compose.ReplyTo, compose.Subject, compose.RemindAt = "", "", "", "", "", ""
	compose.inReplyTo = ""
	compose.references = []string{}
	compose.Attachments = []ComposeAttachment{}
	compose.RequestReadReceipt, compose.TrackLinks = false, false
	compose.SchedulePickerOpen, compose.ReminderPickerOpen = false, false
}

func (compose *ComposeService) mayAutosave() bool {
	if compose.Ready && !compose.Loading && !compose.intentionalExit && compose.ScheduledEditID == "" {
		return true
	}
	return false
}

func (compose *ComposeService) requestDraftSave(allowed bool) {
	if allowed {
		compose.publish(DraftSaveRequested{Envelope: compose.envelope(), DraftID: compose.DraftID})
	}
}

func (compose *ComposeService) exitRoute() string {
	if compose.ScheduledEditID != "" {
		return compose.scheduledStatus()
	}
	return compose.inboxRoute()
}
