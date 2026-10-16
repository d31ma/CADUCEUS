import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// MessageService holds the one open message of the reader: loading it, its
// header and toolbar actions, replies, the protected HTML body, plain text,
// original source, attachments, remote images, link warnings, unsubscribe
// and the state of on-device translation. It opens the message of a
// /email/<id> page, or the one the mailbox list selected (MailSelected).
// Mailbox actions go to MailboxService (MailActionRequested,
// SpamReportRequested); replies hand off to the composer through
// caduceus_compose_prefill; downloads and new tabs click hidden links.
//
// @Service
type MessageService struct {
	// @Needs
	inboxClient *InboxClient
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher

	Email                     EmailDetail
	HasEmail                  bool
	Standalone                bool
	Loading                   bool
	LoadError                 string
	CurrentFolder             string
	IsDraft                   bool
	CanMove                   bool
	CanArchive                bool
	CanMoveToInbox            bool
	CanTrash                  bool
	CanSnooze                 bool
	CanDeleteForever          bool
	CanUnsubscribe            bool
	UnsubscribeState          string
	VisibleSender             string
	ReplyTarget               string
	SenderInitials            string
	DetailedHeaderRows        []HeaderRow
	HasDetailedHeaders        bool
	HasTrackingInsights       bool
	HasTrackingEvents         bool
	ShowOpenedBadge           bool
	HasAttachments            bool
	ShowOriginalRecipient     bool
	ParticipantAddress        string
	AttachmentPreviewsEnabled bool
	RenderMode                string
	ShowQuotedText            bool
	HasQuotedText             bool
	VisiblePlainText          string
	QuotedPlainText           string
	PlainTextVisible          string
	ProtectedDocument         string
	ProtectedRevision         int
	BlockedRemoteContentCount int
	RemoteContentSummary      string
	RemoteImagesLoading       bool
	RemoteImageError          string
	RemotePreferenceSaving    bool
	ImageSender               string
	PendingLink               PendingLinkWarning
	HasPendingLink            bool
	SourceLoading             bool
	SourceError               string
	RawSourceText             string
	AttachmentPreviewOpen     bool
	AttachmentPreviewKind     string
	AttachmentPreviewUrl      string
	AttachmentPreviewText     string
	AttachmentPreviewError    string
	AttachmentPreview         EmailAttachment
	DownloadUrl               string
	DownloadName              string
	ExternalUrl               string
	PrintEmail                string
	Account                   string
	TranslationTarget         string
	TranslationSource         string
	TranslationBusy           bool
	TranslationProgress       float64
	TranslationError          string
	TranslationVisible        bool
	HasTranslation            bool
	TranslationOfferVisible   bool
	TranslationPreference     string
	TranslationStatus         string

	token            string
	pendingID        string
	theme            string
	preferences      MailPreferences
	findOpen         bool
	remoteVisible    bool
	remotePolicy     string
	trustedSenders   []string
	remoteSources    map[string]string
	remoteUrls       []string
	inlineSources    map[string]string
	thumbnails       map[string]string
	attachments      map[string]AttachmentContent
	quotedCount      int
	rawSourceBase64  string
	rawSourceName    string
	translatedHtml   string
	translatedPlain  string
	translationParts map[string][]string
}

// @OnStart
func (service *MessageService) Start() {
	service.preferences = AppearanceUtils{}.DefaultMail()
	service.TranslationTarget = "en"
	service.reset()
}

// HearSession keeps the token and opens the message the page names.
// @OnEvent({"replay":true})
func (service *MessageService) HearSession(changed SessionChanged) {
	service.token = changed.Token
	service.Account = AddressUtils{}.Normalized(changed.Email)
	service.openPending()
	service.settle()
}

// HearLocation reads the message of a /email/<id> page.
// @OnEvent({"replay":true})
func (service *MessageService) HearLocation(place PageLocation) {
	service.Standalone = RouteUtils{}.IsStandalone(place.Path)
	service.pendingID = RouteUtils{}.EmailId(place.Path, place.Search)
	service.openPending()
	service.settle()
}

// HearStorage restores the mail preferences and the translation language.
// @OnEvent({"replay":true})
func (service *MessageService) HearStorage(stored StoredValues) {
	service.adoptPreferences(stored.Session)
	service.TranslationTarget = TextUtils{}.FirstOf(stored.Local[service.names().TargetKey], service.TranslationTarget)
}

// HearStorageWritten follows the mail preferences PreferenceService saves.
// @OnEvent
func (service *MessageService) HearStorageWritten(written StorageWritten) {
	service.adoptPreferences(written.Values)
}

// HearAttributes follows the theme, for the "match theme" message colours.
// @OnEvent({"replay":true})
func (service *MessageService) HearAttributes(changed DocumentAttributesChanged) {
	service.theme = TextUtils{}.FirstOf(changed.Attributes["data-theme"], service.theme)
	service.refreshDocument()
}

// HearSelection opens the message the mailbox list selected.
// @OnEvent
func (service *MessageService) HearSelection(selected MailSelected) {
	service.pendingID = selected.EmailID
	service.openPending()
	service.settle()
}

// HearRefresh reads the open message's folder, labels and flags again after
// a mailbox action, its undo or a spam report.
// @OnEvent
func (service *MessageService) HearRefresh(refresh InboxRefreshRequested) {
	FlowUtils{}.When(LogicUtils{}.All(service.HasEmail, refresh.Reason != service.names().Owner), service.reloadFlags)()
	service.settle()
}

// HearTimer marks the message read when its delay has passed.
// @OnEvent
func (service *MessageService) HearTimer(elapsed TimerElapsed) {
	FlowUtils{}.When(LogicUtils{}.All(elapsed.Name == service.names().MarkReadTimer, service.HasEmail, !service.Email.Read), service.markRead)()
	service.settle()
}

// HearShortcut runs the reader's keyboard commands.
// @OnEvent
func (service *MessageService) HearShortcut(pressed ShortcutPressed) {
	service.runShortcut(pressed.Command)
	service.settle()
}

// HearConfirmation runs the action the person confirmed.
// @OnEvent
func (service *MessageService) HearConfirmation(confirmation ConfirmationGiven) {
	actions := map[string]func(){service.names().DeleteAction: service.removeEmail, service.names().UnsubscribeAction: service.unsubscribeNow}
	action, found := actions[confirmation.Action]
	FlowUtils{}.When(LogicUtils{}.All(found, service.HasEmail, confirmation.ID == service.Email.ID), action)()
	service.settle()
}

// @OnEvent
func (service *MessageService) HearFind(toggled FindToggled) {
	service.findOpen = toggled.Open
}

// HearSourceRequest downloads the original of one conversation message.
// @OnEvent
func (service *MessageService) HearSourceRequest(requested MessageSourceRequested) {
	source, failure := service.inboxClient.Source(service.bearer(), requested.EmailId)
	service.offerConversationSource(source, failure)
	service.settle()
}

// HearUnsubscribeAnswer records whether the one-click POST went out.
// @OnEvent
func (service *MessageService) HearUnsubscribeAnswer(answered UnsubscribeAnswered) {
	service.UnsubscribeState = TextUtils{}.Choose(answered.Sent, service.names().Success, "error")
	service.toast(TextUtils{}.Choose(answered.Sent, "Unsubscribe request sent.", "Could not send the unsubscribe request."))
}

// HearLinkWarning asks before opening a link that disguises its destination.
// @OnEvent
func (service *MessageService) HearLinkWarning(warning PendingLinkWarning) {
	safe := LinkDestinationUtils{}.SafeExternalTarget(warning.Target)
	service.PendingLink = PendingLinkWarning{Reason: TextUtils{}.FirstOf(warning.Reason, "This link may disguise its destination."), Target: safe, DestinationHost: TextUtils{}.FirstOf(warning.DestinationHost, UrlUtils{}.ParseWeb(safe).Host), DisplayedHost: warning.DisplayedHost}
	service.HasPendingLink = safe != ""
}

// HearBackdrop closes the attachment preview or the link warning on a click
// outside it.
// @OnEvent
func (service *MessageService) HearBackdrop(clicked BackdropClicked) {
	closers := map[string]func(){"reader-attachment": service.closeAttachmentPreview, "reader-link": service.CancelPendingLink}
	closer, found := closers[clicked.Dialog]
	FlowUtils{}.When(found, closer)()
}

// @OnEvent
func (service *MessageService) Settled(_ ViewSettled) {}

// LoadEmail reads the open message again.
func (service *MessageService) LoadEmail() {
	service.load()
}

// GoBack returns to the list: /inbox from a message page, or the list beside
// the reader.
func (service *MessageService) GoBack() {
	service.leave()
}

func (service *MessageService) ToggleRead() {
	service.act(map[string]any{service.names().Read: !service.Email.Read}, TextUtils{}.Choose(service.Email.Read, service.names().MarkedUnread, service.names().MarkedRead))
}

func (service *MessageService) ToggleStar() {
	service.act(map[string]any{"starred": !service.Email.Starred}, TextUtils{}.Choose(service.Email.Starred, "Star removed.", "Starred."))
}

func (service *MessageService) Archive() {
	service.act(map[string]any{service.names().FolderKey: service.names().Archive}, "Archived.")
}

func (service *MessageService) MoveToInbox() {
	service.act(map[string]any{service.names().FolderKey: service.names().Inbox}, "Moved to inbox.")
}

func (service *MessageService) MoveToTrash() {
	service.act(map[string]any{service.names().FolderKey: service.names().Trash}, "Moved to trash.")
}

// ReportSpam and MarkNotSpam go to MailboxService, which reports and toasts.
func (service *MessageService) ReportSpam() {
	service.applicationEventPublisher.PublishEvent(SpamReportRequested{ID: service.Email.ID, Spam: true}, false)
}

func (service *MessageService) MarkNotSpam() {
	service.applicationEventPublisher.PublishEvent(SpamReportRequested{ID: service.Email.ID, Spam: false}, false)
}

// OpenPicker opens the shared folder or label picker for this message.
func (service *MessageService) OpenPicker(mode string) {
	message := PickerMessage{ID: service.Email.ID, Domain: service.Email.Domain, Folder: service.Email.Folder, Labels: append([]string{}, service.Email.Labels...)}
	service.applicationEventPublisher.PublishEvent(CategoryPickerRequested{Mode: mode, Messages: []PickerMessage{message}}, false)
}

func (service *MessageService) OpenGitHub() {
	service.openGitHub()
}

func (service *MessageService) EditDraft() {
	service.compose(ComposeFields{DraftID: service.Email.ID})
}

func (service *MessageService) PreferredReply() {
	map[bool]func(){true: service.replyAll, false: service.reply}[service.preferences.DefaultReplyAction == "reply-all"]()
}

func (service *MessageService) ReplyAll() {
	service.replyAll()
}

func (service *MessageService) Forward() {
	service.forward()
}

func (service *MessageService) SetRenderMode(mode string) {
	allowed := map[string]bool{service.names().Html: service.Email.HtmlBody != "", service.names().Text: true, service.names().Source: service.Email.HasRawSource}
	service.RenderMode = TextUtils{}.Choose(allowed[mode], mode, service.RenderMode)
	FlowUtils{}.When(service.RenderMode == service.names().Source, service.loadRawSource)()
	service.refreshText()
}

func (service *MessageService) DownloadOriginal() {
	service.loadRawSource()
	service.offerSource()
}

func (service *MessageService) ToggleQuotedText() {
	service.ShowQuotedText = service.HasQuotedText != service.ShowQuotedText
	service.refreshDocument()
}

func (service *MessageService) ShowRemoteImages() {
	service.remoteVisible = true
	service.refreshDocument()
	service.loadRemoteImages()
}

func (service *MessageService) AlwaysShowImagesFromSender() {
	FlowUtils{}.When(LogicUtils{}.All(service.ImageSender != "", !service.RemotePreferenceSaving), service.trustSender)()
}

func (service *MessageService) CancelPendingLink() {
	service.PendingLink = PendingLinkWarning{}
	service.HasPendingLink = false
}

func (service *MessageService) OpenPendingLink() {
	service.openExternal(LinkDestinationUtils{}.SafeExternalTarget(service.PendingLink.Target))
	service.CancelPendingLink()
}

func (service *MessageService) PreviewAttachment(attachmentID string) {
	service.previewAttachment(service.attachment(attachmentID))
}

func (service *MessageService) DownloadAttachment(attachmentID string) {
	attachment := service.attachment(attachmentID)
	content, failure := service.attachmentContent(attachment.Id)
	service.offerAttachment(content, failure, TextUtils{}.FirstOf(content.Filename, TextUtils{}.FirstOf(attachment.Filename, "attachment")))
}

func (service *MessageService) CloseAttachmentPreview() {
	service.closeAttachmentPreview()
}

// ChangeTranslationTarget remembers the language and shows the original
// until a translation into it exists.
func (service *MessageService) ChangeTranslationTarget(target string) {
	service.TranslationTarget = target
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: map[string]string{service.names().TargetKey: target}, Removed: []string{}}, false)
	service.clearTranslation()
	service.applicationEventPublisher.PublishEvent(TranslationRestoreRequested{MessageId: service.Email.ID, Target: target}, false)
	service.refreshDocument()
	service.refreshTranslation()
}

func (service *MessageService) TranslateMessage() {
	service.translate()
	service.refreshTranslation()
}

func (service *MessageService) ShowTranslatedMessage() {
	service.showTranslation(service.hasTranslatedText())
	service.refreshDocument()
	service.refreshTranslation()
}

func (service *MessageService) ShowOriginalMessage() {
	service.showTranslation(false)
	service.refreshDocument()
	service.refreshTranslation()
}

// HearDetection offers, or starts, a translation of the detected language.
// @OnEvent
func (service *MessageService) HearDetection(detected TranslationDetected) {
	accepted := LogicUtils{}.All(detected.MessageId == service.Email.ID, detected.Language != "", detected.Confidence >= service.names().Confidence, detected.Language != service.TranslationTarget)
	service.TranslationSource = TextUtils{}.Choose(accepted, detected.Language, service.TranslationSource)
	service.TranslationPreference = TextUtils{}.Choose(accepted, detected.Preference, service.TranslationPreference)
	FlowUtils{}.When(LogicUtils{}.All(accepted, detected.Preference == "always"), service.translate)()
	service.refreshTranslation()
}

// HearTranslationPreference follows a choice from the translation menu.
// @OnEvent
func (service *MessageService) HearTranslationPreference(changed TranslationPreferenceChanged) {
	service.TranslationPreference = changed.Preference
	FlowUtils{}.When(changed.Translate, service.translate)()
	FlowUtils{}.When(changed.Original, service.ShowOriginalMessage)()
	service.refreshTranslation()
}

// @OnEvent
func (service *MessageService) HearTranslationProgress(progressed TranslationProgressed) {
	service.TranslationProgress = progressed.Progress
	service.refreshTranslation()
}

// HearTranslationPart collects a translated body, which arrives in parts.
// @OnEvent
func (service *MessageService) HearTranslationPart(part TranslationPart) {
	valid := LogicUtils{}.All(part.MessageId == service.Email.ID, part.Count > 0, part.Index >= 0, part.Index < part.Count)
	FlowUtils{}.When(valid, func() {
		service.translationParts[part.Kind] = TranslationPartUtils{}.With(service.translationParts[part.Kind], part)
	})()
	FlowUtils{}.When(LogicUtils{}.All(valid, part.Kind == "done"), func() { service.finishTranslation(part.Text) })()
	service.refreshDocument()
	service.refreshTranslation()
}

// @OnEvent
func (service *MessageService) HearTranslationFailure(failed TranslationFailed) {
	current := failed.MessageId == service.Email.ID
	service.TranslationError = TextUtils{}.Choose(current, TextUtils{}.FirstOf(failed.Message, "Translation failed."), service.TranslationError)
	service.TranslationBusy = LogicUtils{}.All(service.TranslationBusy, !current)
	service.TranslationProgress = 0
	service.refreshTranslation()
}

func (MessageService) names() ReaderNames {
	return ReaderNames{
		Owner: "message", Inbox: "inbox", FolderKey: "folder", Html: "html", Text: "text", Source: "source",
		MarkReadTimer: "reader-mark-read", DeleteAction: "reader-delete", UnsubscribeAction: "reader-unsubscribe",
		TargetKey: "caduceus-translation-target", MailKey: "caduceus_mail_preferences", Allow: "allow", Sent: "sent",
		Reply: "reply", RePrefix: "Re: ", MarkedRead: "Marked read.", MarkedUnread: "Marked unread.", Archive: "archive", Trash: "trash",
		Cc: "cc", To: "to", Dark: "dark", Original: "message.eml", Read: "read", Success: "success", Confidence: 0.3,
	}
}

func (service *MessageService) bearer() string {
	return "Bearer " + service.token
}

func (service *MessageService) settle() {
	service.applicationEventPublisher.PublishEvent(ViewSettled{Source: service.names().Owner}, false)
}

func (service *MessageService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *MessageService) adoptPreferences(values map[string]string) {
	text, found := values[service.names().MailKey]
	FlowUtils{}.When(found, func() { service.preferences = AppearanceUtils{}.Mail(text) })()
	service.applyThumbnails()
	service.refreshLabels()
}

func (service *MessageService) reset() {
	service.Email = EmailDetail{}
	service.HasEmail = false
	service.LoadError = ""
	service.UnsubscribeState = "idle"
	service.RenderMode = service.names().Html
	service.resetRemoteContent()
	service.resetSource()
	service.resetTranslation()
	service.closeAttachmentPreview()
	service.refreshLabels()
}

func (service *MessageService) resetRemoteContent() {
	service.remoteVisible = false
	service.remotePolicy = "block"
	service.trustedSenders = []string{}
	service.remoteSources = map[string]string{}
	service.remoteUrls = []string{}
	service.inlineSources = map[string]string{}
	service.thumbnails = map[string]string{}
	service.attachments = map[string]AttachmentContent{}
	service.replaceProtectedDocument("")
	service.BlockedRemoteContentCount = 0
	service.RemoteContentSummary = ""
	service.RemoteImagesLoading = false
	service.RemoteImageError = ""
	service.quotedCount = 0
	service.CancelPendingLink()
}

func (service *MessageService) resetSource() {
	service.SourceLoading = false
	service.SourceError = ""
	service.RawSourceText = ""
	service.rawSourceBase64 = ""
	service.rawSourceName = ""
}

func (service *MessageService) resetTranslation() {
	service.TranslationSource = ""
	service.TranslationPreference = "offer"
	service.TranslationBusy = false
	service.TranslationProgress = 0
	service.clearTranslation()
}

func (service *MessageService) clearTranslation() {
	service.TranslationVisible = false
	service.TranslationError = ""
	service.translatedHtml = ""
	service.translatedPlain = ""
	service.translationParts = map[string][]string{}
}

// openPending opens the named message once a token exists, and not again
// while it is already open.
func (service *MessageService) openPending() {
	FlowUtils{}.When(LogicUtils{}.All(service.token != "", service.pendingID != "", service.pendingID != service.Email.ID), service.openFresh)()
}

func (service *MessageService) openFresh() {
	service.reset()
	service.Email = EmailDetail{ID: service.pendingID}
	service.HasEmail = true
	service.load()
}

func (service *MessageService) load() {
	service.Loading = true
	email, failure := service.inboxClient.Read(service.bearer(), service.Email.ID)
	service.LoadError = TextUtils{}.Choose(failure == nil, "", HTTPUtils{}.Message(failure, "Could not load this email.", "Network error"))
	FlowUtils{}.When(HTTPUtils{}.Refused(failure), service.checkSession)()
	FlowUtils{}.When(failure == nil, func() { service.show(email) })()
	service.Loading = false
}

func (service *MessageService) checkSession() {
	service.applicationEventPublisher.PublishEvent(SessionCheckRequested{Reason: "reader"}, false)
}

func (service *MessageService) show(email EmailDetail) {
	service.Email = EmailRecordUtils{}.Email(email)
	service.ShowQuotedText = MessageConversationUtils{}.IsForwardedSubject(service.Email.Subject)
	service.RenderMode = TextUtils{}.Choose(service.Email.HtmlBody != "", service.names().Html, service.names().Text)
	service.prepareAttachments(service.rasterAttachments(service.Email.Attachments))
	service.refreshLabels()
	service.announce()
	service.applicationEventPublisher.PublishEvent(TranslationRestoreRequested{MessageId: service.Email.ID, Target: service.TranslationTarget, Detect: !service.fromCurrentUser()}, false)
	FlowUtils{}.When(service.Email.HtmlBody != "", service.prepareRemoteContent)()
	FlowUtils{}.When(LogicUtils{}.All(!service.Email.Read, service.preferences.MarkAsReadDelay >= 0), service.scheduleMarkRead)()
}

func (service *MessageService) announce() {
	service.applicationEventPublisher.PublishEvent(OpenMessageChanged{
		EmailId:            service.Email.ID,
		Domain:             service.Email.Domain,
		Folder:             service.Email.Folder,
		SystemFolder:       service.Email.SystemFolder,
		Labels:             service.Email.Labels,
		Subject:            service.Email.Subject,
		ParticipantAddress: service.ParticipantAddress,
		Standalone:         service.Standalone,
		CalendarInvite:     service.Email.CalendarInvite,
		Account:            service.Account,
	}, true)
}

func (service *MessageService) refreshLabels() {
	email := service.Email
	service.CurrentFolder = TextUtils{}.FirstOf(email.SystemFolder, email.Folder)
	service.refreshActions(service.HasEmail, email.Folder)
	service.CanUnsubscribe = UnsubscribeUtils{}.BuildAction(email).Type != ""
	service.VisibleSender = TextUtils{}.Choose(LogicUtils{}.All(service.preferences.DetailedNames, email.HeaderFrom != ""), email.HeaderFrom, email.Sender)
	service.ReplyTarget = ReplyUtils{}.Target(email.ReplyTo, email.Sender)
	service.SenderInitials = TextUtils{}.Choose(email.Sender == "", "", AddressUtils{}.Initials(email.Sender))
	service.DetailedHeaderRows = service.headerRows(service.preferences.DetailedHeaders)
	service.HasDetailedHeaders = len(service.DetailedHeaderRows) > 0
	service.HasTrackingInsights = LogicUtils{}.All(service.CurrentFolder == service.names().Sent, LogicUtils{}.Any(email.OpenTracking, email.LinkTracking))
	service.HasTrackingEvents = len(email.TrackingEvents) > 0
	service.ShowOpenedBadge = LogicUtils{}.All(email.OpenedAt != "", !service.HasTrackingInsights)
	service.HasAttachments = len(email.Attachments) > 0
	service.ShowOriginalRecipient = LogicUtils{}.All(email.OriginalRecipient != "", email.OriginalRecipient != email.Recipient)
	service.ParticipantAddress = AddressUtils{}.Participant(email.Sender, append([]string{email.Recipient}, EmailRecordUtils{}.Addresses(service.Email.To)...), service.Account)
	service.AttachmentPreviewsEnabled = service.preferences.DisplayAttachmentPreviews
	service.ImageSender = AddressUtils{}.Valid(AddressUtils{}.Normalized(email.Sender))
	service.PrintEmail = service.printable()
	service.refreshText()
	service.refreshTranslation()
}

// refreshActions decides which toolbar actions this folder offers.
func (service *MessageService) refreshActions(shown bool, folder string) {
	names := service.names()
	service.IsDraft = service.CurrentFolder == "drafts"
	service.CanMove = LogicUtils{}.All(shown, service.Email.Domain != "")
	service.CanArchive = LogicUtils{}.All(shown, folder != names.Archive, folder != names.Trash)
	service.CanMoveToInbox = LogicUtils{}.All(shown, folder != names.Inbox)
	service.CanTrash = LogicUtils{}.All(shown, folder != names.Trash)
	service.CanSnooze = LogicUtils{}.All(shown, service.CurrentFolder == names.Inbox)
	service.CanDeleteForever = LogicUtils{}.All(shown, service.CurrentFolder == names.Trash)
}

// printable is the message as print-conversation.js reads it.
func (service *MessageService) printable() string {
	encoded, _ := json.Marshal(map[string]any{
		"id": service.Email.ID, "subject": service.Email.Subject, "sender": service.Email.Sender, "recipient": service.Email.Recipient,
		service.names().To: EmailRecordUtils{}.Addresses(service.Email.To), service.names().Cc: EmailRecordUtils{}.Addresses(service.Email.Cc), "receivedAt": service.Email.ReceivedAt,
		"body": service.Email.Body, "htmlBody": service.Email.HtmlBody,
	})
	return string(encoded)
}

func (MessageService) listSeparator() string { return ", " }

func (service *MessageService) headerRows(detailed bool) []HeaderRow {
	rows := []HeaderRow{
		{Label: "From", Value: TextUtils{}.FirstOf(service.Email.HeaderFrom, service.Email.Sender)},
		{Label: "To", Value: TextUtils{}.FirstOf(strings.Join(EmailRecordUtils{}.Addresses(service.Email.To), service.listSeparator()), service.Email.Recipient)},
		{Label: "Cc", Value: strings.Join(EmailRecordUtils{}.Addresses(service.Email.Cc), service.listSeparator())},
		{Label: "Reply-To", Value: strings.Join(service.Email.ReplyTo, service.listSeparator())},
		{Label: "Message-ID", Value: TextUtils{}.FirstOf(service.Email.InternetMessageID, service.Email.MessageID)},
	}
	return slices.DeleteFunc(rows, func(row HeaderRow) bool { return LogicUtils{}.Any(row.Value == "", !detailed) })
}

func (service *MessageService) refreshText() {
	parts := MessageConversationUtils{}.SplitQuotedText(service.Email.Body)
	service.PlainTextVisible = parts.Visible
	service.QuotedPlainText = parts.Quoted
	service.VisiblePlainText = service.plainText(parts, service.translatedPlainVisible())
	service.HasQuotedText = map[string]bool{service.names().Html: service.quotedCount > 0, service.names().Text: parts.HasQuoted}[service.RenderMode]
}

func (service *MessageService) plainText(parts QuotedText, translated bool) string {
	switch {
	case translated:
		return service.translatedPlain
	case MessageConversationUtils{}.HasText(parts):
		return parts.Visible
	}
	return "(No body)"
}

// act hands a mailbox action to MailboxService, which offers its undo, and
// shows the change at once.
func (service *MessageService) act(patch map[string]any, message string) {
	FlowUtils{}.When(service.HasEmail, func() {
		service.applicationEventPublisher.PublishEvent(MailActionRequested{Ids: []string{service.Email.ID}, Patch: patch, Message: message}, false)
		service.merge(service.shownPatch(patch))
	})()
}

// shownPatch is the change as the reader shows it before the server answers:
// a move also leaves the system folder, which the next read fills again.
func (service *MessageService) shownPatch(patch map[string]any) map[string]any {
	shown := maps.Clone(patch)
	_, moved := patch[service.names().FolderKey]
	shown["systemFolder"] = map[bool]any{true: "", false: service.Email.SystemFolder}[moved]
	return shown
}

func (service *MessageService) merge(patch map[string]any) {
	service.showChanged(EmailRecordUtils{}.Merged(service.Email, patch))
}

// showChanged shows a changed copy of the open message.
func (service *MessageService) showChanged(email EmailDetail) {
	service.Email = EmailRecordUtils{}.Email(email)
	service.applyThumbnails()
	service.refreshLabels()
	service.announce()
}

// reloadFlags reads the message again and keeps only its mailbox state, so
// the open view does not reset.
func (service *MessageService) reloadFlags() {
	fresh, failure := service.inboxClient.Read(service.bearer(), service.Email.ID)
	flags := service.Email
	flags.Folder, flags.SystemFolder, flags.Labels, flags.Read, flags.Starred = fresh.Folder, fresh.SystemFolder, fresh.Labels, fresh.Read, fresh.Starred
	flags.SpamScore, flags.SpamReasons = fresh.SpamScore, fresh.SpamReasons
	FlowUtils{}.When(failure == nil, func() { service.showChanged(flags) })()
}

func (service *MessageService) removeEmail() {
	_, failure := service.inboxClient.Delete(service.bearer(), service.Email.ID)
	FlowUtils{}.Either(failure == nil, service.removed, func() { service.toast("Delete failed.") })()
}

func (service *MessageService) removed() {
	service.toast("Message deleted.")
	service.refreshInbox()
	service.leave()
	service.reset()
}

func (service *MessageService) refreshInbox() {
	service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: service.names().Owner}, false)
}

func (service *MessageService) leave() {
	FlowUtils{}.Either(service.Standalone, func() {
		service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: "/" + service.names().Inbox}, false)
	}, func() {
		service.applicationEventPublisher.PublishEvent(MailClosed{EmailID: service.Email.ID}, false)
	})()
}

// unsubscribeNow runs the validated unsubscribe path: a mailto draft, a web
// page in a new tab, or the one-click POST.
func (service *MessageService) unsubscribeNow() {
	action := UnsubscribeUtils{}.BuildAction(service.Email)
	handlers := map[string]func(UnsubscribeAction){"compose": service.composeUnsubscribe, "open": service.openUnsubscribe, "post": service.postUnsubscribe}
	handler, found := handlers[action.Type]
	FlowUtils{}.When(LogicUtils{}.All(found, !slices.Contains([]string{"loading", service.names().Success}, service.UnsubscribeState)), func() { handler(action) })()
}

func (service *MessageService) composeUnsubscribe(action UnsubscribeAction) {
	service.compose(ComposeFields{To: strings.Join(action.To, service.listSeparator()), Subject: action.Subject, Body: action.Body})
}

func (service *MessageService) openUnsubscribe(action UnsubscribeAction) {
	service.openExternal(action.Url)
}

func (service *MessageService) postUnsubscribe(action UnsubscribeAction) {
	service.UnsubscribeState = "loading"
	service.applicationEventPublisher.PublishEvent(UnsubscribeRequested{Url: action.Url}, false)
}

// openExternal opens a web address in a new tab through a hidden link.
func (service *MessageService) openExternal(target string) {
	service.ExternalUrl = target
	FlowUtils{}.When(target != "", func() {
		service.applicationEventPublisher.PublishEvent(ClickRequested{Selector: "[data-message-external]"}, false)
	})()
}

func (service *MessageService) openGitHub() {
	service.openExternal(TextUtils{}.Choose(service.findOpen, "", GitHubNotificationUtils{}.SafeTarget(service.Email.GithubUrl)))
}

// compose hands the fields to the composer: caduceus_compose_prefill in
// sessionStorage, then /compose.
func (service *MessageService) compose(fields ComposeFields) {
	FlowUtils{}.When(service.HasEmail, func() {
		service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Stored(fields), false)
		service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Opened(), false)
	})()
}

func (service *MessageService) reply() {
	fields := service.threadFields()
	fields.To = service.ReplyTarget
	fields.Subject = service.names().RePrefix + service.Email.Subject
	fields.Quote = service.quote(service.names().Reply)
	fields.Body = service.quoted(service.names().Reply)
	service.compose(fields)
}

func (service *MessageService) replyAll() {
	fields := service.threadFields()
	fields.To = strings.Join(TextUtils{}.NonEmpty([]string{service.ReplyTarget, service.Email.Recipient}), service.listSeparator())
	fields.Subject = service.names().RePrefix + service.Email.Subject
	fields.Quote = service.quote(service.names().Reply)
	fields.Body = service.quoted(service.names().Reply)
	service.compose(fields)
}

func (service *MessageService) forward() {
	mode := "forward"
	service.compose(ComposeFields{Subject: "Fwd: " + service.Email.Subject, Body: service.quoted(mode), Quote: service.quote(mode), SourceEmailID: service.Email.ID})
}

func (service *MessageService) quoted(mode string) string {
	return QuotedMessageUtils{}.Html(*service.quote(mode))
}

// quote captures the actual reader record before HTML enters the prefill.
func (service *MessageService) quote(mode string) *QuotedMessage {
	return &QuotedMessage{Mode: mode, Sender: service.Email.Sender, Subject: service.Email.Subject, DateLabel: service.Email.ReceivedLabel, Body: service.Email.Body}
}

// threadFields keep a reply in the message's RFC thread.
func (service *MessageService) threadFields() ComposeFields {
	parent := service.Email.InternetMessageID
	return ComposeFields{InReplyTo: parent, References: ReplyUtils{}.References(service.Email.References, parent), SourceEmailID: service.Email.ID}
}

func (service *MessageService) scheduleMarkRead() {
	service.applicationEventPublisher.PublishEvent(TimerRequested{Name: service.names().MarkReadTimer, DelayMs: service.preferences.MarkAsReadDelay}, false)
}

func (service *MessageService) markRead() {
	_, failure := service.inboxClient.Update(service.bearer(), service.Email.ID, map[string]any{service.names().Read: true})
	FlowUtils{}.When(failure == nil, func() {
		service.merge(map[string]any{service.names().Read: true})
		service.refreshInbox()
	})()
}

// runShortcut runs a reader command. The list beside the reader already
// acts on its selection for the shared mailbox commands, so the reader takes
// them only on its own page.
func (service *MessageService) runShortcut(command string) {
	actions := map[string]func(){"email:reply": service.PreferredReply, "email:reply-all": service.replyAll, "email:forward": service.forward, "github:open": service.openGitHub}
	FlowUtils{}.When(service.Standalone, func() { maps.Copy(actions, service.mailboxShortcuts()) })()
	action, found := actions[command]
	FlowUtils{}.When(LogicUtils{}.All(found, service.HasEmail), action)()
}

func (service *MessageService) mailboxShortcuts() map[string]func() {
	return map[string]func(){
		"email:star": service.ToggleStar, "email:archive": service.Archive, "email:trash": service.MoveToTrash, "email:report-spam": service.ReportSpam,
		"email:mark-read":   func() { service.act(map[string]any{service.names().Read: true}, service.names().MarkedRead) },
		"email:mark-unread": func() { service.act(map[string]any{service.names().Read: false}, service.names().MarkedUnread) },
		"email:open": func() {
			service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Message(service.Email.ID), false)
		},
	}
}

func (service *MessageService) loadRawSource() {
	FlowUtils{}.When(LogicUtils{}.All(service.rawSourceBase64 == "", !service.SourceLoading, service.HasEmail), service.fetchRawSource)()
}

func (service *MessageService) fetchRawSource() {
	service.SourceLoading = true
	source, failure := service.inboxClient.Source(service.bearer(), service.Email.ID)
	bytes, decodeFailure := base64.StdEncoding.DecodeString(source.ContentBase64)
	loaded := LogicUtils{}.All(failure == nil, decodeFailure == nil)
	service.SourceError = TextUtils{}.Choose(loaded, "", "Could not load the original message source.")
	service.rawSourceBase64 = TextUtils{}.Choose(loaded, source.ContentBase64, "")
	service.rawSourceName = TextUtils{}.FirstOf(source.Filename, service.names().Original)
	service.RawSourceText = strings.ToValidUTF8(string(bytes), "�")
	service.SourceLoading = false
}

func (MessageService) originalData() string { return "data:message/rfc822;base64," }

func (service *MessageService) offerSource() {
	FlowUtils{}.Either(service.rawSourceBase64 == "", func() {
		service.toast(TextUtils{}.FirstOf(service.SourceError, "Original message source is unavailable."))
	}, func() { service.download(service.originalData()+service.rawSourceBase64, service.rawSourceName) })()
}

// download saves a data address through the hidden download link.
func (service *MessageService) download(address string, filename string) {
	service.DownloadUrl = address
	service.DownloadName = filename
	service.applicationEventPublisher.PublishEvent(ClickRequested{Selector: "[data-message-download]"}, false)
}

func (service *MessageService) offerConversationSource(source MessageSource, failure error) {
	FlowUtils{}.Either(HTTPUtils{}.Missed(failure, source.ContentBase64), func() { service.toast("Original message is unavailable.") }, func() {
		service.download(service.originalData()+source.ContentBase64, TextUtils{}.FirstOf(source.Filename, service.names().Original))
	})()
}

func (service *MessageService) prepareRemoteContent() {
	preferences, failure := service.usersClient.Preferences(service.bearer())
	FlowUtils{}.When(failure == nil, func() { service.imagePreferences(preferences) })()
	service.refreshDocument()
	FlowUtils{}.When(service.shouldLoadRemote(), service.loadRemoteImages)()
}

func (service *MessageService) imagePreferences(preferences UserPreferences) {
	service.remotePolicy = TextUtils{}.Choose(preferences.RemoteImagePolicy == service.names().Allow, service.names().Allow, "block")
	service.trustedSenders = AddressUtils{}.NormalizedAll(preferences.TrustedImageSenders)
}

// shouldLoadRemote: shown on request; never for spam; always when the
// policy allows; otherwise only for trusted senders.
func (service *MessageService) shouldLoadRemote() bool {
	switch {
	case service.remoteVisible:
		return true
	case service.Email.Spam:
		return false
	case service.remotePolicy == service.names().Allow:
		return true
	}
	return AddressUtils{}.Trusted(service.ImageSender, service.trustedSenders)
}

func (service *MessageService) refreshDocument() {
	document := ContentProtectionUtils{}.BuildDocument(service.documentBody(), ProtectionOptions{
		AllowRemote:   service.shouldLoadRemote(),
		InlineSources: service.inlineSources,
		RemoteSources: service.remoteSources,
		ShowQuoted:    service.ShowQuotedText,
		ColorMode:     TextUtils{}.Choose(service.preferences.EmailRenderMode == "theme", TextUtils{}.Choose(service.theme == service.names().Dark, service.names().Dark, "light"), service.preferences.EmailRenderMode),
	})
	service.replaceProtectedDocument(document.DocumentHtml)
	service.BlockedRemoteContentCount = document.BlockedCount
	service.RemoteContentSummary = RemoteImageUtils{}.Summary(document.BlockedCount, document.TrackingCount)
	service.quotedCount = document.QuotedCount
	service.remoteUrls = document.RemoteUrls
	service.refreshText()
}

func (service *MessageService) replaceProtectedDocument(document string) {
	if service.ProtectedDocument != document {
		service.ProtectedRevision++
	}
	service.ProtectedDocument = document
}

func (service *MessageService) documentBody() string {
	return TextUtils{}.Choose(LogicUtils{}.All(service.TranslationVisible, service.translatedHtml != ""), service.translatedHtml, service.Email.HtmlBody)
}

func (service *MessageService) loadRemoteImages() {
	urls := slices.DeleteFunc(slices.Clone(service.remoteUrls), func(address string) bool { return service.remoteSources[address] != "" })
	FlowUtils{}.When(LogicUtils{}.All(len(urls) > 0, !service.RemoteImagesLoading, service.HasEmail), func() { service.fetchRemoteImages(urls) })()
}

func (service *MessageService) fetchRemoteImages(urls []string) {
	service.RemoteImagesLoading = true
	service.RemoteImageError = RemoteImageUtils{}.Failure(service.fetchImages(urls))
	service.refreshDocument()
	service.RemoteImagesLoading = false
}

// fetchImages asks the proxy for one image at a time: an answer may hold at
// most 1 MiB.
func (service *MessageService) fetchImages(urls []string) int {
	failures := 0
	for _, address := range urls {
		images, failure := service.inboxClient.Images(service.bearer(), service.Email.ID, map[string]any{"urls": []string{address}})
		failures += RemoteImageUtils{}.Failures(images, failure, 1)
		RemoteImageUtils{}.Keep(service.remoteSources, images.Images)
	}
	return failures
}

func (service *MessageService) trustSender() {
	sender := service.ImageSender
	service.RemotePreferenceSaving = true
	saved, failure := service.usersClient.SavePreferences(service.bearer(), map[string]any{"trustedImageSenders": append(slices.Clone(service.trustedSenders), sender)})
	service.RemotePreferenceSaving = false
	FlowUtils{}.Either(failure == nil, func() {
		service.trustedSenders = AddressUtils{}.NormalizedAll(saved.TrustedImageSenders)
		service.ShowRemoteImages()
		service.toast("Images from " + sender + " will load automatically.")
	}, func() { service.toast("Could not save the image preference.") })()
}

func (service *MessageService) rasterAttachments(attachments []EmailAttachment) []EmailAttachment {
	return slices.DeleteFunc(slices.Clone(attachments), func(attachment EmailAttachment) bool {
		return !AttachmentContentUtils{}.IsRasterImage(attachment.ContentType)
	})
}

// prepareAttachments loads the image attachments for thumbnails and for the
// cid: images of the HTML body.
func (service *MessageService) prepareAttachments(rasters []EmailAttachment) {
	for _, attachment := range rasters {
		content, failure := service.attachmentContent(attachment.Id)
		service.thumbnails[attachment.Id] = TextUtils{}.Choose(failure == nil, AttachmentContentUtils{}.PreviewDataUrl(content.ContentType, content.ContentBase64), "")
		service.inlineSources = AttachmentContentUtils{}.WithInlineSource(service.inlineSources, attachment.ContentId, service.thumbnails[attachment.Id])
	}
	service.applyThumbnails()
}

func (service *MessageService) applyThumbnails() {
	for index := range service.Email.Attachments {
		service.Email.Attachments[index].ThumbnailUrl = TextUtils{}.Choose(service.preferences.DisplayAttachmentPreviews, service.thumbnails[service.Email.Attachments[index].Id], "")
	}
}

// attachment finds an attachment of the message or of its invitation; an
// unknown id gives an attachment with only that id.
func (service *MessageService) attachment(attachmentID string) EmailAttachment {
	known := append(slices.Clone(service.Email.Attachments), EmailRecordUtils{}.InviteAttachments(service.Email)...)
	found := slices.IndexFunc(known, func(attachment EmailAttachment) bool { return attachment.Id == attachmentID })
	return append([]EmailAttachment{{Id: attachmentID}}, known...)[found+1]
}

// attachmentContent reads an attachment once and keeps it for the visit.
func (service *MessageService) attachmentContent(attachmentID string) (AttachmentContent, error) {
	_, kept := service.attachments[attachmentID]
	FlowUtils{}.When(!kept, func() { service.fetchAttachment(attachmentID) })()
	return service.keptAttachment(attachmentID)
}

func (service *MessageService) fetchAttachment(attachmentID string) {
	content, failure := service.inboxClient.Attachment(service.bearer(), service.Email.ID, attachmentID)
	FlowUtils{}.When(LogicUtils{}.All(failure == nil, content.Id == attachmentID, content.ContentBase64 != ""), func() { service.attachments[attachmentID] = content })()
}

func (service *MessageService) keptAttachment(attachmentID string) (AttachmentContent, error) {
	content, kept := service.attachments[attachmentID]
	if !kept {
		return AttachmentContent{}, fmt.Errorf("Attachment could not be loaded.")
	}
	return content, nil
}

func (service *MessageService) previewAttachment(attachment EmailAttachment) {
	kind := AttachmentContentUtils{}.PreviewKind(attachment.ContentType)
	switch {
	case !service.preferences.DisplayAttachmentPreviews:
		service.toast("Attachment previews are disabled in Mail preferences.")
	case kind != "":
		service.openPreview(attachment, kind)
	}
}

func (service *MessageService) openPreview(attachment EmailAttachment, kind string) {
	service.closeAttachmentPreview()
	service.AttachmentPreviewOpen = true
	service.AttachmentPreviewKind = kind
	service.AttachmentPreview = attachment
	content, failure := service.attachmentContent(attachment.Id)
	text, textFailure := AttachmentContentUtils{}.DecodeText(content.ContentBase64)
	address := AttachmentContentUtils{}.PreviewDataUrl(content.ContentType, content.ContentBase64)
	service.AttachmentPreviewError = AttachmentContentUtils{}.PreviewFailure(kind, address, failure, textFailure)
	shown := service.AttachmentPreviewError == ""
	textual := kind == service.names().Text
	service.AttachmentPreviewText = TextUtils{}.Choose(LogicUtils{}.All(shown, textual), text, "")
	service.AttachmentPreviewUrl = TextUtils{}.Choose(LogicUtils{}.All(shown, !textual), address, "")
}

func (service *MessageService) closeAttachmentPreview() {
	service.AttachmentPreviewOpen = false
	service.AttachmentPreviewKind = ""
	service.AttachmentPreviewUrl = ""
	service.AttachmentPreviewText = ""
	service.AttachmentPreviewError = ""
	service.AttachmentPreview = EmailAttachment{}
}

func (service *MessageService) offerAttachment(content AttachmentContent, failure error, filename string) {
	FlowUtils{}.Either(LogicUtils{}.Any(failure != nil, !service.HasEmail), func() { service.toast("Attachment download failed.") }, func() {
		service.download("data:"+TextUtils{}.FirstOf(content.ContentType, "application/octet-stream")+";base64,"+content.ContentBase64, filename)
	})()
}

// fromCurrentUser: the user's own messages are never offered a translation.
func (service *MessageService) fromCurrentUser() bool {
	sender := AddressUtils{}.Normalized(service.Email.Sender)
	return LogicUtils{}.Any(strings.ToLower(service.CurrentFolder) == service.names().Sent, LogicUtils{}.All(sender != "", sender == service.Account))
}

func (service *MessageService) translate() {
	FlowUtils{}.When(LogicUtils{}.All(service.HasEmail, !service.TranslationBusy), service.requestTranslation)()
}

func (service *MessageService) requestTranslation() {
	service.TranslationBusy = true
	service.TranslationProgress = 0
	service.TranslationError = ""
	service.translationParts = map[string][]string{}
	service.applicationEventPublisher.PublishEvent(TranslationRequested{MessageId: service.Email.ID, Source: service.TranslationSource, Target: service.TranslationTarget}, false)
}

func (service *MessageService) finishTranslation(source string) {
	service.translatedHtml = strings.Join(service.translationParts[service.names().Html], "")
	service.translatedPlain = strings.Join(service.translationParts[service.names().Text], "")
	service.TranslationSource = TextUtils{}.FirstOf(source, service.TranslationSource)
	service.TranslationBusy = false
	service.TranslationProgress = 0
	service.showTranslation(service.hasTranslatedText())
}

func (service *MessageService) hasTranslatedText() bool {
	return LogicUtils{}.Any(service.translatedHtml != "", service.translatedPlain != "")
}

func (service *MessageService) showTranslation(visible bool) {
	service.TranslationVisible = visible
	service.applicationEventPublisher.PublishEvent(TranslationVisibilityChanged{MessageId: service.Email.ID, Enabled: visible}, false)
}

func (service *MessageService) refreshTranslation() {
	service.HasTranslation = service.hasTranslatedText()
	active := LogicUtils{}.Any(service.TranslationBusy, service.TranslationVisible, service.HasTranslation, service.TranslationError != "")
	pointless := LogicUtils{}.Any(service.TranslationSource == "", service.TranslationSource == service.TranslationTarget, service.fromCurrentUser())
	service.TranslationOfferVisible = LogicUtils{}.Any(active, LogicUtils{}.All(!pointless, service.TranslationPreference != "never"))
	service.TranslationStatus = service.translationStatus(LogicUtils{}.All(service.TranslationProgress > 0, service.TranslationProgress < 1))
	service.VisiblePlainText = service.plainText(MessageConversationUtils{}.SplitQuotedText(service.Email.Body), service.translatedPlainVisible())
}

func (service *MessageService) translatedPlainVisible() bool {
	return LogicUtils{}.All(service.TranslationVisible, service.translatedPlain != "")
}

func (service *MessageService) translationStatus(downloading bool) string {
	switch {
	case !service.TranslationBusy:
		return ""
	case downloading:
		return fmt.Sprintf("Downloading language model… %d%%", int(service.TranslationProgress*100+0.5))
	}
	return "Translating on this device…"
}
