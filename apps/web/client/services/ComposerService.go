import "strings"

// @Service
type ComposerService struct {
	Target          SendTarget
	NewConversation bool
	ShowCc          bool
	ShowBlindCopy   bool
	Subject         string
	Body            string
	To              []Recipient
	Cc              []Recipient
	BlindCopy       []Recipient
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

// @OnEvent
func (composer *ComposerService) Picked(picked UploadPicked) {
	composer.uploads.Pick(picked)
	composer.refresh()
}

// @OnEvent
func (composer *ComposerService) Uploaded(finished UploadFinished) {
	composer.uploads.Complete(finished.AttachmentId)
	composer.refresh()
}

// @OnEvent
func (composer *ComposerService) DomainChanged(changed SessionDomainChanged) {
	composer.clear()
	composer.Target = SendTarget{}
	composer.NewConversation = true
	composer.refresh()
}

func (composer *ComposerService) AddTyped(role string, text string) {
	composer.recipients.Add(text, role)
	composer.refresh()
}

func (composer *ComposerService) SendWith(body string) {
	composer.Body = body
	composer.startSending()
}

func (composer *ComposerService) KeepSubject(subject string) {
	composer.Subject = subject
}

func (composer *ComposerService) Remove(address string, role string) {
	composer.recipients.Remove(address, role)
	composer.refresh()
}

func (composer *ComposerService) Choose(address string, role string, delivery string) {
	composer.recipients.Choose(address, role, delivery)
	composer.Announcement = composer.choiceAnnouncement(composer.recipients.Find(address))
	composer.refresh()
}

func (composer *ComposerService) ToggleCc() {
	composer.ShowCc = !composer.ShowCc
	composer.refresh()
}

func (composer *ComposerService) ToggleBlindCopy() {
	composer.ShowBlindCopy = !composer.ShowBlindCopy
	composer.refresh()
}

func (composer *ComposerService) startSending() {
	composer.recipients.ResolvePending()
	composer.Problem = composer.firstProblem(composer.checks())
	composer.SubjectMissing = composer.Problem == composer.problemSubjectMissing()
	composer.deliverWhenClear(composer.request(composer.emailMode()))
	composer.refresh()
}

func (composer *ComposerService) CloseChanged() {
	composer.ChangedOpen = false
	composer.Changes = []RecipientChange{}
}

func (composer *ComposerService) RetryUpload(fileName string) {
	composer.uploads.Retry(fileName)
	composer.refresh()
}

func (composer *ComposerService) RemoveUpload(fileName string) {
	composer.uploads.Remove(fileName)
	composer.refresh()
}

func (composer *ComposerService) refresh() {
	composer.To = composer.recipients.InRole(composer.roleTo())
	composer.Cc = composer.recipients.InRole(composer.roleCc())
	composer.BlindCopy = composer.recipients.InRole(composer.roleBlindCopy())
	composer.ShowCc = composer.rowShown(composer.ShowCc, composer.Cc)
	composer.ShowBlindCopy = composer.rowShown(composer.ShowBlindCopy, composer.BlindCopy)
	composer.Rows = composer.rows()
	composer.Uploads = composer.uploads.Items
	composer.UploadsUsed = composer.uploads.UsedText()
	composer.Summary = composer.recipients.Summary()
	composer.SendLabel = composer.recipients.SendLabel()
	composer.EmailMode = composer.emailMode()
	composer.ModeText = composer.recipients.ModeText()
	composer.SenderAddress = composer.recipients.SenderAddress()
	composer.ProblemText = composer.problems.Text(composer.Problem)
}

func (composer *ComposerService) emailMode() bool {
	return composer.recipients.EmailMode() || composer.Target.Subject != ""
}

func (composer *ComposerService) rows() []RecipientRow {
	return []RecipientRow{
		{Role: composer.roleTo(), Label: "To", Placeholder: composer.toPlaceholder(), Recipients: composer.To, Shown: composer.NewConversation},
		{Role: composer.roleCc(), Label: "Cc", Placeholder: "Add people who get a copy", Recipients: composer.Cc, Shown: composer.shownWithNew(composer.ShowCc)},
		{Role: composer.roleBlindCopy(), Label: "Bcc", Placeholder: "Hidden from everyone else", Recipients: composer.BlindCopy, Shown: composer.shownWithNew(composer.ShowBlindCopy)},
	}
}

func (composer *ComposerService) rowShown(shown bool, recipients []Recipient) bool {
	return shown || len(recipients) > 0
}

func (composer *ComposerService) shownWithNew(shown bool) bool {
	return composer.NewConversation && shown
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

func (composer *ComposerService) choiceAnnouncement(chosen Recipient) string {
	return chosen.DisplayName + " now gets: " + chosen.Label + "."
}

func (composer *ComposerService) checks() []string {
	return []string{
		composer.needsRecipient(),
		composer.needsFewerRecipients(),
		composer.needsValidRecipients(),
		composer.needsResolvedRecipients(),
		composer.needsSubject(composer.recipients.EmailMode()),
		composer.needsContent(),
		composer.needsFinishedUploads(),
		composer.needsWorkingUploads(),
	}
}

func (composer *ComposerService) firstProblem(problems []string) string {
	found := strings.Fields(strings.Join(problems, composer.separator()))
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

func (composer *ComposerService) needsFewerRecipients() string {
	if len(composer.recipients.Recipients) <= composer.mostRecipients() {
		return ""
	}
	return "tooManyRecipients"
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

func (composer *ComposerService) needsSubject(emailMode bool) string {
	if !composer.NewConversation || !emailMode || strings.TrimSpace(composer.Subject) != "" {
		return ""
	}
	return composer.problemSubjectMissing()
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

func (composer *ComposerService) deliverWhenClear(request SendRequest) {
	if composer.Problem != "" {
		return
	}
	composer.deliver(request)
}

func (composer *ComposerService) deliver(request SendRequest) {
	result, failure := composer.send.Send(request)
	composer.Problem = composer.problems.OfFailure(failure)
	composer.afterSend(result)
}

func (composer *ComposerService) request(emailMode bool) SendRequest {
	return SendRequest{
		ConversationId: composer.Target.ConversationId,
		ThreadRootId:   composer.Target.ThreadRootId,
		Body:           strings.TrimSpace(composer.Body),
		AttachmentIds:  composer.uploads.AttachmentIds(),
		Recipients:     composer.recipients.Choices(),
		Email:          composer.emailOptions(emailMode),
	}
}

func (composer *ComposerService) emailOptions(emailMode bool) *EmailOptions {
	if !emailMode {
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
	composer.ShowCc = false
	composer.ShowBlindCopy = false
	composer.Problem = ""
	composer.SubjectMissing = false
}

func (composer *ComposerService) mostRecipients() int {
	return 20
}

func (composer *ComposerService) problemSubjectMissing() string {
	return "subjectMissing"
}

func (composer *ComposerService) roleTo() string {
	return "to"
}

func (composer *ComposerService) roleCc() string {
	return "cc"
}

func (composer *ComposerService) roleBlindCopy() string {
	return "bcc"
}

func (composer *ComposerService) separator() string {
	return " "
}
