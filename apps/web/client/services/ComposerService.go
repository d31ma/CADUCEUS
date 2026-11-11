import "strings"

// @Service
type ComposerService struct {
	Target          SendTarget
	NewConversation bool
	ToText          string
	CcText          string
	BccText         string
	ShowCc          bool
	ShowBcc         bool
	Subject         string
	Body            string
	To              []Recipient
	Cc              []Recipient
	Bcc             []Recipient
	Rows            []RecipientRow
	Uploads         []UploadItem
	UploadsUsed     string
	Summary         []string
	SendLabel       string
	EmailMode       bool
	ModeText        string
	SenderAddress   string
	Problem         string
	ProblemText     string
	SubjectMissing  bool
	Sending         bool
	ChangedOpen     bool
	Changes         []RecipientChange
	Announcement    string
	// @Needs
	recipients *ComposerRecipientService
	// @Needs
	send *ComposerSendService
	// @Needs
	problems *ComposerProblemService
	// @Needs
	uploads                   *ComposerUploadService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (composer *ComposerService) Start() {
	composer.NewConversation = true
	composer.refresh()
}

// @OnEvent
func (composer *ComposerService) Open(target SendTarget) {
	composer.Target = target
	composer.NewConversation = target.ConversationId == ""
	composer.Subject = composer.replySubject(target.Subject)
	composer.refresh()
}

func (composer *ComposerService) StartNew() {
	composer.Open(SendTarget{})
}

func (composer *ComposerService) AddTyped(role string) {
	composer.recipients.Add(composer.textOf(role), role)
	composer.setText(role, "")
	composer.refresh()
}

func (composer *ComposerService) RecipientKey(role string, key string, value string) {
	composer.setText(role, value)
	composer.keyAction(key, value)(role)
	composer.refresh()
}

func (composer *ComposerService) TypeText(role string, value string) {
	composer.setText(role, value)
}

func (composer *ComposerService) Remove(address string, role string) {
	composer.recipients.Remove(address, role)
	composer.refresh()
}

func (composer *ComposerService) Choose(address string, role string, delivery string) {
	composer.recipients.Choose(address, role, delivery)
	composer.Announcement = composer.choiceAnnouncement(address)
	composer.refresh()
}

func (composer *ComposerService) ToggleMenu(address string, role string) {
	composer.recipients.ToggleMenu(address, role)
	composer.refresh()
}

func (composer *ComposerService) CloseMenus() {
	composer.recipients.CloseMenus()
	composer.refresh()
}

func (composer *ComposerService) ToggleCc() {
	composer.ShowCc = !composer.ShowCc
	composer.refresh()
}

func (composer *ComposerService) ToggleBcc() {
	composer.ShowBcc = !composer.ShowBcc
	composer.refresh()
}

func (composer *ComposerService) BodyKey(key string, control bool, command bool) {
	composer.sendWhenShortcut(composer.isSendShortcut(key, control, command))
}

func (composer *ComposerService) Send() {
	composer.recipients.ResolvePending()
	composer.Problem = composer.firstProblem(composer.checks())
	composer.SubjectMissing = composer.Problem == "subjectMissing"
	composer.requestWhenClear()
	composer.refresh()
}

// @OnEvent
// @InBackground
func (composer *ComposerService) Deliver(requested SendRequested) {
	result, failure := composer.send.Send(requested.Request)
	composer.Sending = false
	composer.Problem = composer.problems.OfFailure(failure)
	composer.afterSend(result)
	composer.refresh()
}

func (composer *ComposerService) CloseChanged() {
	composer.ChangedOpen = false
	composer.Changes = []RecipientChange{}
}

func (composer *ComposerService) Pick(picked UploadPicked) {
	composer.uploads.Pick(picked)
	composer.refresh()
}

func (composer *ComposerService) RetryUpload(fileName string) {
	composer.uploads.Retry(fileName)
	composer.refresh()
}

func (composer *ComposerService) RemoveUpload(fileName string) {
	composer.uploads.Remove(fileName)
	composer.refresh()
}

func (composer *ComposerService) Discard() {
	composer.clear()
	composer.refresh()
}

func (composer *ComposerService) refresh() {
	composer.To = composer.recipients.InRole("to")
	composer.Cc = composer.recipients.InRole("cc")
	composer.Bcc = composer.recipients.InRole("bcc")
	composer.ShowCc = composer.rowShown(composer.ShowCc, composer.Cc)
	composer.ShowBcc = composer.rowShown(composer.ShowBcc, composer.Bcc)
	composer.Uploads = composer.uploads.Items
	composer.UploadsUsed = composer.uploads.UsedText()
	composer.Summary = composer.recipients.Summary()
	composer.SendLabel = composer.recipients.SendLabel()
	composer.EmailMode = composer.emailMode()
	composer.Rows = composer.rows()
	composer.ModeText = composer.recipients.ModeText()
	composer.SenderAddress = composer.recipients.SenderAddress()
	composer.ProblemText = composer.problems.Text(composer.Problem)
}

func (composer *ComposerService) emailMode() bool {
	return composer.recipients.EmailMode() || composer.Target.Subject != ""
}

func (composer *ComposerService) rows() []RecipientRow {
	rows := []RecipientRow{}
	rows = composer.appendRow(rows, composer.NewConversation, RecipientRow{Role: "to", Label: "To", Placeholder: composer.toPlaceholder(), Text: composer.ToText, Recipients: composer.To})
	rows = composer.appendRow(rows, composer.NewConversation && composer.ShowCc, RecipientRow{Role: "cc", Label: "Cc", Placeholder: "Add people who get a copy", Text: composer.CcText, Recipients: composer.Cc})
	return composer.appendRow(rows, composer.NewConversation && composer.ShowBcc, RecipientRow{Role: "bcc", Label: "Bcc", Placeholder: "Hidden from everyone else", Text: composer.BccText, Recipients: composer.Bcc})
}

func (composer *ComposerService) appendRow(rows []RecipientRow, shown bool, row RecipientRow) []RecipientRow {
	if !shown {
		return rows
	}
	return append(rows, row)
}

func (composer *ComposerService) toPlaceholder() string {
	if len(composer.To) == 0 {
		return "Name, channel or email address"
	}
	return "Add more people"
}

func (composer *ComposerService) replySubject(subject string) string {
	if subject == "" || strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

func (composer *ComposerService) textOf(role string) string {
	texts := map[string]string{"to": composer.ToText, "cc": composer.CcText, "bcc": composer.BccText}
	return texts[role]
}

func (composer *ComposerService) setText(role string, value string) {
	fields := map[string]*string{"to": &composer.ToText, "cc": &composer.CcText, "bcc": &composer.BccText}
	*fields[role] = value
}

func (composer *ComposerService) keyAction(key string, value string) func(string) {
	switch {
	case composer.addingKeys()[key]:
		return composer.addTypedOnly
	case key == "Backspace" && value == "":
		return composer.recipients.RemoveLast
	}
	return composer.ignoreKey
}

func (composer *ComposerService) addTypedOnly(role string) {
	composer.recipients.Add(composer.textOf(role), role)
	composer.setText(role, "")
}

func (composer *ComposerService) ignoreKey(role string) {
}

func (composer *ComposerService) choiceAnnouncement(address string) string {
	chosen := composer.recipients.Find(address)
	return chosen.DisplayName + " now gets: " + chosen.Label + "."
}

func (composer *ComposerService) sendWhenShortcut(pressed bool) {
	if !pressed {
		return
	}
	composer.Send()
}

func (composer *ComposerService) checks() []string {
	return []string{
		composer.needsIdle(),
		composer.needsRecipient(),
		composer.needsValidRecipients(),
		composer.needsResolvedRecipients(),
		composer.needsSubject(),
		composer.needsContent(),
		composer.needsFinishedUploads(),
		composer.needsWorkingUploads(),
	}
}

func (composer *ComposerService) firstProblem(problems []string) string {
	found := strings.Fields(strings.Join(problems, " "))
	if len(found) == 0 {
		return ""
	}
	return found[0]
}

func (composer *ComposerService) needsRecipient() string {
	if !composer.NewConversation || len(composer.recipients.Recipients) > 0 {
		return ""
	}
	return "addRecipient"
}

func (composer *ComposerService) needsValidRecipients() string {
	if composer.recipients.CountInState("invalid") == 0 {
		return ""
	}
	return "fixRecipients"
}

func (composer *ComposerService) needsResolvedRecipients() string {
	if composer.recipients.CountInState("checking") == 0 {
		return ""
	}
	return "waitForCheck"
}

func (composer *ComposerService) needsSubject() string {
	if !composer.NewConversation || !composer.recipients.EmailMode() || strings.TrimSpace(composer.Subject) != "" {
		return ""
	}
	return "subjectMissing"
}

func (composer *ComposerService) needsContent() string {
	if strings.TrimSpace(composer.Body) != "" || len(composer.uploads.AttachmentIds()) > 0 {
		return ""
	}
	return "emptyMessage"
}

func (composer *ComposerService) needsFinishedUploads() string {
	if composer.uploads.CountInState("uploading") == 0 {
		return ""
	}
	return "waitForUploads"
}

func (composer *ComposerService) needsWorkingUploads() string {
	if composer.uploads.CountInState("failed")+composer.uploads.CountInState("refused") == 0 {
		return ""
	}
	return "fixAttachments"
}

func (composer *ComposerService) requestWhenClear() {
	if composer.Problem != "" {
		return
	}
	composer.Sending = true
	composer.applicationEventPublisher.PublishEvent(SendRequested{Request: composer.request()}, false)
}

func (composer *ComposerService) needsIdle() string {
	if !composer.Sending {
		return ""
	}
	return "stillSending"
}

func (composer *ComposerService) isSendShortcut(key string, control bool, command bool) bool {
	return key == "Enter" && (control || command)
}

func (composer *ComposerService) rowShown(shown bool, recipients []Recipient) bool {
	return shown || len(recipients) > 0
}

func (composer *ComposerService) request() SendRequest {
	return SendRequest{
		ConversationId: composer.Target.ConversationId,
		ThreadRootId:   composer.Target.ThreadRootId,
		Body:           strings.TrimSpace(composer.Body),
		AttachmentIds:  composer.uploads.AttachmentIds(),
		Recipients:     composer.recipients.Choices(),
		Email:          composer.emailOptions(),
	}
}

func (composer *ComposerService) emailOptions() *EmailOptions {
	if !composer.emailMode() {
		return nil
	}
	return &EmailOptions{Subject: strings.TrimSpace(composer.Subject)}
}

func (composer *ComposerService) afterSend(result SendResult) {
	switch composer.Problem {
	case "":
		composer.sent(result)
	case "recipientsChanged", "chatNotAllowed":
		composer.showChanges()
	}
}

func (composer *ComposerService) sent(result SendResult) {
	composer.clear()
	composer.Target.ConversationId = result.Message.ConversationId
	composer.NewConversation = false
	composer.Announcement = "Message sent."
	composer.applicationEventPublisher.PublishEvent(SendCompleted{ConversationId: result.Message.ConversationId, MessageId: result.Message.MessageId}, false)
}

func (composer *ComposerService) showChanges() {
	composer.Changes = composer.recipients.Refresh()
	composer.ChangedOpen = true
	composer.Announcement = "Not sent. Some recipients changed."
}

func (composer *ComposerService) clear() {
	composer.recipients.Clear()
	composer.uploads.Clear()
	composer.send.Reset()
	composer.Body = ""
	composer.Subject = ""
	composer.ToText = ""
	composer.CcText = ""
	composer.BccText = ""
	composer.ShowCc = false
	composer.ShowBcc = false
	composer.Problem = ""
	composer.SubjectMissing = false
}

func (composer *ComposerService) addingKeys() map[string]bool {
	return map[string]bool{"Enter": true, ",": true, ";": true, "Tab": true}
}
