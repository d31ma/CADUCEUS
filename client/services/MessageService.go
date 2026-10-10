import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// MessageService holds the one open message of the reader: loading it, its
// header and toolbar actions, replies, the protected HTML body, plain text,
// original source, attachments, remote images, link warnings, unsubscribe
// and the state of on-device translation. Browser-only effects (downloads,
// new windows, timers and the translation engine) go to JavaScript
// services as events.
//
// @Service
type MessageService struct {
	// @Needs
	inboxClient *InboxClient
	// @Needs
	usersClient *UsersClient
	// @Needs
	sendClient                *SendClient
	secureRandom              SecureRandom
	applicationEventPublisher ApplicationEventPublisher

	Email                     Email
	HasEmail                  bool
	Loading                   bool
	LoadError                 string
	CurrentFolder             string
	IsDraft                   bool
	CanUnsubscribe            bool
	UnsubscribeState          string
	ShowReply                 bool
	ReplyText                 string
	VisibleSender             string
	ReplyTarget               string
	SenderInitials            string
	DetailedHeaderRows        []HeaderRow
	HasTrackingInsights       bool
	ParticipantAddress        string
	AttachmentPreviewsEnabled bool
	RenderMode                string
	ShowQuotedText            bool
	HasQuotedText             bool
	VisiblePlainText          string
	QuotedPlainText           string
	PlainTextVisible          string
	ProtectedDocument         string
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
	AttachmentPreviewLoading  bool
	AttachmentPreviewKind     string
	AttachmentPreviewUrl      string
	AttachmentPreviewText     string
	AttachmentPreviewError    string
	AttachmentPreview         EmailAttachment
	DownloadUrl               string
	DownloadName              string
	ComposePrefill            ComposePrefill
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

	record           map[string]any
	token            string
	ownAddress       string
	path             string
	pendingId        string
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
	dateLabels       map[string]string
	translatedHtml   string
	translatedPlain  string
	translationParts map[string][]string
}

// @OnStart
func (service *MessageService) Start() {
	service.preferences = MailPreferencesUtils{}.Defaults()
	service.TranslationTarget = "en"
	service.reset()
}

// @OnEvent({"replay":true})
func (service *MessageService) SessionChanged(changed SessionChanged) error {
	service.token = changed.Token
	service.ownAddress = AddressUtils{}.Normalized(changed.Email)
	return service.openPending()
}

// @OnEvent({"replay":true})
func (service *MessageService) PreferencesChanged(changed MailPreferencesChanged) {
	service.preferences = MailPreferencesUtils{}.Normalize(changed)
	service.applyThumbnails()
	service.refreshLabels()
}

// @OnEvent({"replay":true})
func (service *MessageService) ReaderOpened(opened ReaderOpened) error {
	service.path = opened.Path
	service.theme = opened.Theme
	service.TranslationTarget = TextUtils{}.Or(opened.TranslationTarget, service.TranslationTarget)
	service.pendingId = RouteUtils{}.EmailId(opened.Path, opened.Search)
	return service.openPending()
}

// @OnEvent
func (service *MessageService) EmailSelected(selected EmailSelected) error {
	service.pendingId = selected.EmailId
	failure := service.openPending()
	service.changed()
	return failure
}

// @OnEvent
func (service *MessageService) MessageChanged(changed MessageChanged) {
	service.applyChange(changed)
}

// @OnEvent
func (service *MessageService) FindToggled(toggled FindToggled) {
	service.findOpen = toggled.Open
}

// @OnEvent
func (service *MessageService) SourceRequested(requested MessageSourceRequested) {
	source, failure := service.inboxClient.Source(requested.EmailId, service.authorization())
	service.offerConversationSource(source, failure)
	service.changed()
}

// @OnEvent
func (service *MessageService) Shortcut(shortcut ShortcutTriggered) {
	service.runShortcut(shortcut.Name)
	service.changed()
}

func (service *MessageService) LoadEmail() error {
	return service.load()
}

func (service *MessageService) GoBack() {
	service.goBack(RouteUtils{}.IsStandalone(service.path))
}

func (MessageService) readKey() string { return "read" }

func (MessageService) markedRead() string { return "Marked read." }

func (MessageService) markedUnread() string { return "Marked unread." }

func (service *MessageService) ToggleRead() {
	service.updateEmail(map[string]any{service.readKey(): !service.Email.Read}, TextUtils{}.Choose(service.Email.Read, service.markedUnread(), service.markedRead()))
}

func (MessageService) starredKey() string { return "starred" }

func (service *MessageService) ToggleStar() {
	service.updateEmail(map[string]any{service.starredKey(): !service.Email.Starred}, TextUtils{}.Choose(service.Email.Starred, "Star removed.", "Starred."))
}

func (MessageService) folderKey() string { return "folder" }

func (service *MessageService) Archive() {
	service.updateEmail(map[string]any{service.folderKey(): "archive"}, "Archived.")
}

func (MessageService) inboxFolder() string { return "inbox" }

func (MessageService) movedToInbox() string { return "Moved to inbox." }

func (service *MessageService) MoveToInbox() {
	service.updateEmail(map[string]any{service.folderKey(): service.inboxFolder()}, service.movedToInbox())
}

func (service *MessageService) MoveToTrash() {
	service.updateEmail(map[string]any{service.folderKey(): "trash"}, "Moved to trash.")
}

func (MessageService) spamKey() string { return "spam" }

func (service *MessageService) ReportSpam() {
	raw, failure := service.inboxClient.MarkSpam(service.Email.Id, map[string]any{service.spamKey(): true}, service.authorization())
	service.spamMarked(RecordUtils{}.Object(raw), failure, SpamOutcome{Success: "Reported spam.", Failure: "Report spam failed."})
}

func (service *MessageService) MarkNotSpam() {
	raw, failure := service.inboxClient.MarkSpam(service.Email.Id, map[string]any{service.spamKey(): false}, service.authorization())
	service.spamMarked(RecordUtils{}.Object(raw), failure, SpamOutcome{Success: service.movedToInbox(), Failure: "Not spam failed."})
}

func (service *MessageService) DeleteEmail(confirmed bool) {
	service.deleteEmail(confirmed)
}

func (service *MessageService) Unsubscribe(confirmed bool) {
	service.unsubscribe(confirmed, UnsubscribeUtils{}.BuildAction(service.Email))
}

func (service *MessageService) UnsubscribeFinished(sent bool) {
	service.unsubscribeFinished(sent)
}

func (service *MessageService) OpenGitHub() {
	service.openGitHub()
}

func (service *MessageService) EditDraft() {
	service.compose(ComposePrefill{DraftId: service.Email.Id, References: []string{}})
}

func (service *MessageService) PreferredReply() {
	service.preferredReply(service.preferences.DefaultReplyAction)
}

func (service *MessageService) ReplyAll() {
	service.replyAll()
}

func (service *MessageService) Forward() {
	service.forward()
}

func (service *MessageService) SendReply() {
	service.sendReply(service.threadPrefill(service.Email.InternetMessageId))
}

func (MessageService) htmlMode() string { return "html" }

func (MessageService) textMode() string { return "text" }

func (MessageService) sourceMode() string { return "source" }

func (service *MessageService) SetRenderMode(mode string) {
	allowed := map[string]bool{service.htmlMode(): service.Email.HtmlBody != "", service.textMode(): true, service.sourceMode(): service.Email.HasRawSource}
	service.chooseMode(mode, allowed[mode])
	service.loadSourceFor(service.RenderMode == service.sourceMode(), service.needsSource())
	service.refreshText()
}

func (service *MessageService) DownloadOriginal() {
	service.loadRawSource(service.needsSource())
	service.offerSource()
}

func (service *MessageService) ToggleQuotedText() {
	service.toggleQuotedText()
	service.refreshDocument()
}

func (service *MessageService) ShowRemoteImages() {
	service.remoteVisible = true
	service.refreshDocument()
	service.loadRemoteImages(service.unloadedUrls())
}

func (service *MessageService) AlwaysShowImagesFromSender() {
	service.trustSender(service.ImageSender)
}

func (service *MessageService) ShowLinkWarning(reason string, target string, destinationHost string, displayedHost string) {
	safe := LinkDestinationUtils{}.SafeExternalTarget(target)
	service.showLinkWarning(PendingLinkWarning{Reason: TextUtils{}.Or(reason, "This link may disguise its destination."), Target: safe, DestinationHost: TextUtils{}.Or(destinationHost, UrlUtils{}.ParseWeb(safe).Host), DisplayedHost: displayedHost})
}

func (service *MessageService) CancelPendingLink() {
	service.PendingLink = PendingLinkWarning{}
	service.HasPendingLink = false
}

func (service *MessageService) OpenPendingLink() {
	service.openExternal(LinkDestinationUtils{}.SafeExternalTarget(service.PendingLink.Target))
	service.CancelPendingLink()
}

func (service *MessageService) DismissLinkWarning(className string) {
	service.dismissWhen(className == "email-link-warning-backdrop", service.CancelPendingLink)
}

func (service *MessageService) PreviewAttachment(attachment EmailAttachment) {
	service.previewAttachment(attachment, AttachmentContentUtils{}.PreviewKind(attachment.ContentType))
}

func (service *MessageService) DownloadAttachment(attachment EmailAttachment) {
	content, failure := service.attachmentContent(attachment.Id)
	service.offerAttachment(content, failure, TextUtils{}.Or(content.Filename, TextUtils{}.Or(attachment.Filename, "attachment")))
}

func (service *MessageService) CloseAttachmentPreview() {
	service.closeAttachmentPreview()
}

func (service *MessageService) DismissAttachmentPreview(className string) {
	service.dismissWhen(className == "attachment-preview-backdrop", service.closeAttachmentPreview)
}

func (service *MessageService) TimerFired(name string, key string) {
	service.timerFired(name, key)
	service.changed()
}

func (service *MessageService) AcceptDateLabels(values []string, labels []string) {
	service.acceptDateLabels(values, labels)
}

func (service *MessageService) ChangeTranslationTarget(target string) {
	service.TranslationTarget = target
	service.applicationEventPublisher.PublishEvent(ReaderPreferenceChanged{Name: "translationTarget", Value: target}, false)
	service.clearTranslation()
	service.applicationEventPublisher.PublishEvent(TranslationRestoreRequested{MessageId: service.Email.Id, Target: target}, false)
	service.refreshDocument()
	service.refreshTranslation()
}

func (service *MessageService) TranslateMessage() {
	service.translate()
	service.refreshTranslation()
}

func (service *MessageService) LanguageDetected(messageId string, language string, confidence float64, preference string) {
	accepted := service.detectionAccepted(messageId, language, confidence)
	service.languageDetected(accepted, language, preference)
	service.translateWhen(service.automatic(accepted, preference))
	service.refreshTranslation()
}

func (service *MessageService) TranslationPreferenceChanged(preference string) {
	service.TranslationPreference = preference
	service.refreshTranslation()
}

func (service *MessageService) TranslationProgressed(progress float64) {
	service.TranslationProgress = progress
	service.refreshTranslation()
}

func (service *MessageService) ReceiveTranslation(part TranslationPart) {
	service.receiveTranslation(part)
	service.refreshDocument()
	service.refreshTranslation()
}

func (service *MessageService) TranslationFailed(messageId string, message string) {
	service.translationFailed(messageId == service.Email.Id, TextUtils{}.Or(message, "Translation failed."))
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

func (MessageService) idleState() string { return "idle" }

func (service *MessageService) reset() {
	service.record = map[string]any{}
	service.Email = EmailRecordUtils{}.Email(service.record)
	service.HasEmail = false
	service.UnsubscribeState = service.idleState()
	service.RenderMode = service.htmlMode()
	service.ComposePrefill = ComposePrefill{References: []string{}}
	service.ShowReply = false
	service.ReplyText = ""
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
	service.dateLabels = map[string]string{}
	service.ProtectedDocument = ""
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

func (service *MessageService) authorization() string {
	return "Bearer " + service.token
}

func (service *MessageService) openPending() error {
	if service.token == "" || service.pendingId == "" || (service.pendingId == service.Email.Id && service.Email.Body != "") {
		return nil
	}
	return service.openFresh(service.pendingId)
}

func (service *MessageService) openFresh(emailId string) error {
	service.reset()
	service.record = map[string]any{"id": emailId}
	service.Email = EmailRecordUtils{}.Email(service.record)
	service.HasEmail = true
	failure := service.load()
	service.changed()
	return failure
}

func (service *MessageService) load() error {
	service.Loading = true
	raw, failure := service.inboxClient.Read(service.Email.Id, service.authorization())
	service.LoadError = service.loadFailure(FailureUtils{}.Code(failure))
	service.showLoaded(RecordUtils{}.Object(raw), failure == nil)
	service.Loading = false
	return nil
}

func (service *MessageService) loadFailure(code string) string {
	switch code {
	case "":
		return ""
	case "HTTP_STATUS":
		return "Could not load this email."
	}
	return "Network error"
}

func (service *MessageService) showLoaded(record map[string]any, loaded bool) {
	if loaded {
		service.show(record)
	}
}

func (MessageService) owner() string { return "message" }

func (service *MessageService) show(record map[string]any) {
	service.record = record
	service.Email = EmailRecordUtils{}.Email(record)
	service.ShowQuotedText = MessageConversationUtils{}.IsForwardedSubject(service.Email.Subject)
	service.RenderMode = TextUtils{}.Choose(service.Email.HtmlBody != "", service.htmlMode(), service.textMode())
	service.prepareAttachments(service.rasterAttachments(service.Email.Attachments))
	service.refreshLabels()
	service.announce()
	service.applicationEventPublisher.PublishEvent(TranslationRestoreRequested{MessageId: service.Email.Id, Target: service.TranslationTarget, Detect: !service.fromCurrentUser()}, false)
	service.applicationEventPublisher.PublishEvent(DateLabelsRequested{Owner: service.owner(), Values: []string{service.Email.ReceivedAt}}, false)
	service.prepareHtml(service.Email.HtmlBody != "")
	service.scheduleMarkRead()
}

func (service *MessageService) announce() {
	service.applicationEventPublisher.PublishEvent(OpenMessageChanged{
		EmailId:            service.Email.Id,
		Domain:             service.Email.Domain,
		Folder:             service.Email.Folder,
		SystemFolder:       service.Email.SystemFolder,
		Labels:             service.Email.Labels,
		Subject:            service.Email.Subject,
		ParticipantAddress: service.ParticipantAddress,
		Standalone:         RouteUtils{}.IsStandalone(service.path),
		CalendarInvite:     EmailRecordUtils{}.Invite(service.record),
	}, true)
}

func (service *MessageService) changed() {
	service.applicationEventPublisher.PublishEvent(ReaderChanged{Reason: service.owner()}, false)
}

func (service *MessageService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *MessageService) refreshInbox() {
	service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: service.owner()}, false)
}

func (service *MessageService) refreshLabels() {
	service.CurrentFolder = TextUtils{}.Or(service.Email.SystemFolder, service.Email.Folder)
	service.IsDraft = service.CurrentFolder == "drafts"
	service.CanUnsubscribe = UnsubscribeUtils{}.BuildAction(service.Email).Type != ""
	service.VisibleSender = service.visibleSender()
	service.ReplyTarget = ReplyUtils{}.Target(service.Email.ReplyTo, service.Email.Sender)
	service.SenderInitials = TextUtils{}.Choose(service.Email.Sender == "", "", AddressUtils{}.Initials(service.Email.Sender))
	service.DetailedHeaderRows = service.headerRows(service.preferences.DetailedHeaders, service.headerValues())
	service.HasTrackingInsights = service.trackingInsights()
	service.ParticipantAddress = AddressUtils{}.Participant(service.Email.Sender, append([]string{service.Email.Recipient}, service.Email.To...), service.ownAddress)
	service.AttachmentPreviewsEnabled = service.preferences.DisplayAttachmentPreviews
	service.ImageSender = AddressUtils{}.Valid(AddressUtils{}.Normalized(service.Email.Sender))
	service.refreshText()
	service.refreshTranslation()
}

func (service *MessageService) visibleSender() string {
	if service.preferences.DetailedNames && service.Email.HeaderFrom != "" {
		return service.Email.HeaderFrom
	}
	return service.Email.Sender
}

func (MessageService) listSeparator() string { return ", " }

func (service *MessageService) headerValues() []HeaderRow {
	return []HeaderRow{
		{Label: "From", Value: TextUtils{}.Or(service.Email.HeaderFrom, service.Email.Sender)},
		{Label: "To", Value: TextUtils{}.Or(strings.Join(service.Email.To, service.listSeparator()), service.Email.Recipient)},
		{Label: "Cc", Value: strings.Join(service.Email.Cc, service.listSeparator())},
		{Label: "Reply-To", Value: strings.Join(service.Email.ReplyTo, service.listSeparator())},
		{Label: "Message-ID", Value: TextUtils{}.Or(service.Email.InternetMessageId, service.Email.MessageId)},
	}
}

func (service *MessageService) headerRows(detailed bool, rows []HeaderRow) []HeaderRow {
	if !detailed {
		return []HeaderRow{}
	}
	return slices.DeleteFunc(rows, func(row HeaderRow) bool { return row.Value == "" })
}

func (MessageService) sentFolder() string { return "sent" }

func (service *MessageService) trackingInsights() bool {
	if service.CurrentFolder == service.sentFolder() && (service.Email.OpenTracking || service.Email.LinkTracking) {
		return true
	}
	return false
}

func (service *MessageService) refreshText() {
	parts := MessageConversationUtils{}.SplitQuotedText(service.Email.Body)
	service.PlainTextVisible = parts.Visible
	service.QuotedPlainText = parts.Quoted
	service.VisiblePlainText = service.plainText(parts, service.translatedPlainVisible())
	service.HasQuotedText = service.hasQuoted(parts.HasQuoted)
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

func (service *MessageService) hasQuoted(plainQuoted bool) bool {
	switch service.RenderMode {
	case service.htmlMode():
		return service.quotedCount > 0
	case service.textMode():
		return plainQuoted
	}
	return false
}

func (service *MessageService) updateEmail(patch map[string]any, message string) {
	for _, emailId := range service.openEmailIds() {
		requestId, _ := service.secureRandom.UUID()
		service.performAction(map[string]any{"ids": []string{emailId}, "patch": patch, "clientRequestId": requestId}, OfflineRequestRequested{Id: requestId, Message: message})
	}
}

func (service *MessageService) openEmailIds() []string {
	if service.HasEmail {
		return []string{service.Email.Id}
	}
	return []string{}
}

func (service *MessageService) performAction(body map[string]any, offline OfflineRequestRequested) {
	result, failure := service.inboxClient.Act(body, service.authorization())
	encoded, _ := json.Marshal(body)
	offline.Kind = "mail-action"
	offline.Path = "/inbox/actions"
	offline.Method = "POST"
	offline.Body = string(encoded)
	offline.EmailId = service.Email.Id
	service.actionDone(result, FailureUtils{}.Code(failure), offline)
}

func (service *MessageService) actionDone(result MailActionResult, code string, offline OfflineRequestRequested) {
	switch code {
	case FailureUtils{}.OfflineCode():
		service.applicationEventPublisher.PublishEvent(offline, false)
	case "":
		service.actionApplied(result, offline.Message)
	default:
		service.toast("Mail action failed.")
	}
}

func (service *MessageService) actionApplied(result MailActionResult, message string) {
	for _, value := range result.Emails {
		service.mergeEmail(RecordUtils{}.Object(value))
	}
	service.applicationEventPublisher.PublishEvent(MailActionCompleted{ActionId: result.ActionId, ExpiresAt: result.ExpiresAt, Message: message}, false)
	service.refreshInbox()
}

func (service *MessageService) mergeEmail(record map[string]any) {
	if RecordUtils{}.Text(record, "id") == service.Email.Id {
		service.merge(record)
	}
}

func (service *MessageService) merge(record map[string]any) {
	service.record = RecordUtils{}.Merge(service.record, record)
	service.Email = EmailRecordUtils{}.Email(service.record)
	service.applyThumbnails()
	service.refreshLabels()
	service.announce()
}

func (service *MessageService) applyChange(changed MessageChanged) {
	if changed.EmailId == service.Email.Id {
		service.merge(map[string]any{service.folderKey(): changed.Folder, "systemFolder": changed.SystemFolder, "labels": changed.Labels, service.readKey(): changed.Read, service.starredKey(): changed.Starred})
	}
}

func (service *MessageService) spamMarked(record map[string]any, failure error, outcome SpamOutcome) {
	if failure != nil {
		service.toast(outcome.Failure)
		return
	}
	service.merge(record)
	service.refreshInbox()
	service.toast(outcome.Success)
}

func (service *MessageService) deleteEmail(confirmed bool) {
	if confirmed && service.HasEmail {
		service.removeEmail()
	}
}

func (service *MessageService) removeEmail() {
	_, failure := service.inboxClient.Delete(service.Email.Id, service.authorization())
	service.deleted(failure == nil)
}

func (service *MessageService) deleted(removed bool) {
	if !removed {
		service.toast("Delete failed.")
		return
	}
	service.toast("Message deleted.")
	service.refreshInbox()
	service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: service.inboxFolder()}, false)
	service.reset()
}

func (service *MessageService) goBack(standalone bool) {
	if standalone {
		service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: service.inboxFolder()}, false)
		return
	}
	service.applicationEventPublisher.PublishEvent(MailBackRequested{Reason: service.owner()}, false)
}

func (service *MessageService) unsubscribe(confirmed bool, action UnsubscribeAction) {
	if confirmed && action.Type != "" && service.UnsubscribeState != "loading" && service.UnsubscribeState != "success" {
		service.runUnsubscribe(action)
	}
}

func (service *MessageService) runUnsubscribe(action UnsubscribeAction) {
	handlers := map[string]func(UnsubscribeAction){"compose": service.composeUnsubscribe, "open": service.openUnsubscribe, "post": service.postUnsubscribe}
	handlers[action.Type](action)
}

func (service *MessageService) composeUnsubscribe(action UnsubscribeAction) {
	service.compose(ComposePrefill{To: strings.Join(action.To, service.listSeparator()), Subject: action.Subject, Body: action.Body, References: []string{}})
}

func (service *MessageService) openUnsubscribe(action UnsubscribeAction) {
	service.openExternal(action.Url)
}

func (service *MessageService) postUnsubscribe(action UnsubscribeAction) {
	service.UnsubscribeState = "loading"
	service.applicationEventPublisher.PublishEvent(UnsubscribeRequested{Url: action.Url}, false)
}

func (service *MessageService) unsubscribeFinished(sent bool) {
	if sent {
		service.UnsubscribeState = "success"
		service.toast("Unsubscribe request sent.")
		return
	}
	service.UnsubscribeState = "error"
	service.toast("Could not send the unsubscribe request.")
}

func (service *MessageService) openExternal(target string) {
	if target != "" {
		service.applicationEventPublisher.PublishEvent(ExternalLinkRequested{Url: target}, false)
	}
}

func (service *MessageService) openGitHub() {
	target := GitHubNotificationUtils{}.SafeTarget(service.Email.GithubUrl)
	service.openExternal(TextUtils{}.Choose(service.findOpen, "", target))
}

func (service *MessageService) compose(prefill ComposePrefill) {
	if service.HasEmail {
		service.ComposePrefill = prefill
		service.applicationEventPublisher.PublishEvent(ComposeRequested{Owner: service.owner()}, false)
	}
}

func (service *MessageService) preferredReply(action string) {
	if action == "reply-all" {
		service.replyAll()
		return
	}
	service.reply()
}

func (MessageService) replyMode() string { return "reply" }

func (MessageService) replyPrefix() string { return "Re: " }

func (service *MessageService) reply() {
	prefill := service.threadPrefill(service.Email.InternetMessageId)
	prefill.To = service.ReplyTarget
	prefill.Subject = service.replyPrefix() + service.Email.Subject
	prefill.Body = service.quoted(service.replyMode())
	service.compose(prefill)
}

func (service *MessageService) replyAll() {
	prefill := service.threadPrefill(service.Email.InternetMessageId)
	prefill.To = strings.Join(TextUtils{}.NonEmpty([]string{service.ReplyTarget, service.Email.Recipient}), service.listSeparator())
	prefill.Subject = service.replyPrefix() + service.Email.Subject
	prefill.Body = service.quoted(service.replyMode())
	service.compose(prefill)
}

func (service *MessageService) forward() {
	service.compose(ComposePrefill{Subject: "Fwd: " + service.Email.Subject, Body: service.quoted("forward"), SourceEmailId: service.Email.Id, References: []string{}})
}

func (service *MessageService) quoted(mode string) string {
	return QuotedMessageUtils{}.Html(QuotedMessage{Mode: mode, Sender: service.Email.Sender, Subject: service.Email.Subject, DateLabel: service.dateLabels[service.Email.ReceivedAt], Body: service.Email.Body})
}

func (service *MessageService) threadPrefill(parent string) ComposePrefill {
	if parent == "" {
		return ComposePrefill{SourceEmailId: service.Email.Id, References: []string{}}
	}
	return ComposePrefill{InReplyTo: parent, References: ReplyUtils{}.References(service.Email.References, parent), SourceEmailId: service.Email.Id}
}

func (service *MessageService) sendReply(thread ComposePrefill) {
	if service.HasEmail && service.ReplyText != "" {
		service.deliverReply(thread)
	}
}

func (service *MessageService) deliverReply(thread ComposePrefill) {
	body := map[string]any{"to": []string{service.ReplyTarget}, "subject": service.replyPrefix() + service.Email.Subject, "text": service.ReplyText, "sourceEmailId": thread.SourceEmailId}
	service.addThread(body, thread)
	_, failure := service.sendClient.Send(body, service.authorization())
	service.replySent(FailureUtils{}.Code(failure))
}

func (service *MessageService) addThread(body map[string]any, thread ComposePrefill) {
	if thread.InReplyTo != "" {
		body["inReplyTo"] = thread.InReplyTo
		body["references"] = thread.References
	}
}

func (service *MessageService) replySent(code string) {
	switch code {
	case "":
		service.toast("Reply sent.")
		service.ShowReply = false
		service.ReplyText = ""
	case FailureUtils{}.OfflineCode():
		service.toast("Network error.")
	default:
		service.toast("Send failed.")
	}
}

func (MessageService) markReadTimer() string { return "mark-read" }

func (service *MessageService) scheduleMarkRead() {
	if !service.Email.Read && service.preferences.MarkAsReadDelay >= 0 {
		service.applicationEventPublisher.PublishEvent(TimerRequested{Owner: service.owner(), Name: service.markReadTimer(), Key: service.Email.Id, DelayMs: service.preferences.MarkAsReadDelay}, false)
	}
}

func (service *MessageService) timerFired(name string, key string) {
	if name == service.markReadTimer() && key == service.Email.Id && !service.Email.Read {
		service.markRead()
	}
}

func (service *MessageService) markRead() {
	_, failure := service.inboxClient.Update(service.Email.Id, map[string]any{service.readKey(): true}, service.authorization())
	service.markedReadNow(failure == nil)
}

func (service *MessageService) markedReadNow(updated bool) {
	if updated {
		service.merge(map[string]any{service.readKey(): true})
		service.refreshInbox()
	}
}

func (service *MessageService) runShortcut(name string) {
	actions := map[string]func(){
		"email:reply":       service.PreferredReply,
		"email:reply-all":   service.replyAll,
		"email:forward":     service.forward,
		"email:star":        service.ToggleStar,
		"email:archive":     service.Archive,
		"email:trash":       service.MoveToTrash,
		"email:report-spam": service.ReportSpam,
		"email:mark-read":   service.markReadShortcut,
		"email:mark-unread": service.markUnreadShortcut,
		"email:open":        service.openFull,
		"github:open":       service.openGitHub,
	}
	service.runAction(actions[name])
}

func (service *MessageService) markReadShortcut() {
	service.updateEmail(map[string]any{service.readKey(): true}, service.markedRead())
}

func (service *MessageService) markUnreadShortcut() {
	service.updateEmail(map[string]any{service.readKey(): false}, service.markedUnread())
}

func (service *MessageService) runAction(action func()) {
	if action != nil && service.HasEmail {
		action()
	}
}

func (service *MessageService) openFull() {
	service.applicationEventPublisher.PublishEvent(EmailOpenRequested{EmailId: service.Email.Id}, false)
}

func (service *MessageService) chooseMode(mode string, allowed bool) {
	if allowed {
		service.RenderMode = mode
	}
}

func (service *MessageService) loadSourceFor(isSource bool, needed bool) {
	if isSource && needed {
		service.fetchRawSource()
	}
}

func (service *MessageService) needsSource() bool {
	if service.rawSourceBase64 == "" && !service.SourceLoading && service.HasEmail {
		return true
	}
	return false
}

func (service *MessageService) loadRawSource(needed bool) {
	if needed {
		service.fetchRawSource()
	}
}

func (service *MessageService) fetchRawSource() {
	service.SourceLoading = true
	source, failure := service.inboxClient.Source(service.Email.Id, service.authorization())
	bytes, decodeFailure := base64.StdEncoding.DecodeString(source.ContentBase64)
	service.sourceLoaded(source, bytes, []error{failure, decodeFailure})
	service.SourceLoading = false
}

func (MessageService) originalFilename() string { return "message.eml" }

func (service *MessageService) sourceLoaded(source MessageSource, bytes []byte, failures []error) {
	if failures[0] != nil || failures[1] != nil {
		service.SourceError = "Could not load the original message source."
		return
	}
	service.SourceError = ""
	service.rawSourceBase64 = source.ContentBase64
	service.rawSourceName = TextUtils{}.Or(source.Filename, service.originalFilename())
	service.RawSourceText = strings.ToValidUTF8(string(bytes), "�")
}

func (MessageService) originalData() string { return "data:message/rfc822;base64," }

func (service *MessageService) offerSource() {
	if service.rawSourceBase64 == "" {
		service.toast(TextUtils{}.Or(service.SourceError, "Original message source is unavailable."))
		return
	}
	service.download(service.originalData()+service.rawSourceBase64, service.rawSourceName)
}

func (service *MessageService) download(address string, filename string) {
	service.DownloadUrl = address
	service.DownloadName = filename
	service.applicationEventPublisher.PublishEvent(DownloadRequested{Owner: service.owner()}, false)
}

func (service *MessageService) offerConversationSource(source MessageSource, failure error) {
	if failure != nil || source.ContentBase64 == "" {
		service.toast("Original message is unavailable.")
		return
	}
	service.download(service.originalData()+source.ContentBase64, TextUtils{}.Or(source.Filename, service.originalFilename()))
}

func (service *MessageService) toggleQuotedText() {
	if service.HasQuotedText {
		service.ShowQuotedText = !service.ShowQuotedText
	}
}

func (service *MessageService) dismissWhen(onBackdrop bool, close func()) {
	if onBackdrop {
		close()
	}
}

func (service *MessageService) showLinkWarning(warning PendingLinkWarning) {
	if warning.Target != "" {
		service.PendingLink = warning
		service.HasPendingLink = true
	}
}

func (service *MessageService) prepareHtml(hasHtml bool) {
	if hasHtml {
		service.prepareRemoteContent()
	}
}

func (service *MessageService) prepareRemoteContent() {
	preferences, failure := service.usersClient.Preferences(service.authorization())
	service.imagePreferences(preferences, failure == nil)
	service.refreshDocument()
	service.loadRemoteImages(service.allowedUrls(service.shouldLoadRemote()))
}

func (MessageService) allowPolicy() string { return "allow" }

func (service *MessageService) imagePreferences(preferences ImagePreferences, loaded bool) {
	if loaded {
		service.remotePolicy = TextUtils{}.Choose(preferences.RemoteImagePolicy == service.allowPolicy(), service.allowPolicy(), "block")
		service.trustedSenders = AddressUtils{}.NormalizedAll(preferences.TrustedImageSenders)
	}
}

func (service *MessageService) shouldLoadRemote() bool {
	return service.remoteDecision(AddressUtils{}.Trusted(service.ImageSender, service.trustedSenders))
}

func (service *MessageService) remoteDecision(trusted bool) bool {
	switch {
	case service.remoteVisible:
		return true
	case service.Email.Spam:
		return false
	case service.remotePolicy == service.allowPolicy():
		return true
	}
	return trusted
}

func (service *MessageService) allowedUrls(allowed bool) []string {
	if allowed {
		return service.unloadedUrls()
	}
	return []string{}
}

func (service *MessageService) refreshDocument() {
	document := ContentProtectionUtils{}.BuildDocument(service.documentBody(), ProtectionOptions{
		AllowRemote:   service.shouldLoadRemote(),
		InlineSources: service.inlineSources,
		RemoteSources: service.remoteSources,
		ShowQuoted:    service.ShowQuotedText,
		ColorMode:     service.colorMode(service.preferences.EmailRenderMode),
	})
	service.ProtectedDocument = document.DocumentHtml
	service.BlockedRemoteContentCount = document.BlockedCount
	service.RemoteContentSummary = RemoteImageUtils{}.Summary(document.BlockedCount, document.TrackingCount)
	service.quotedCount = document.QuotedCount
	service.remoteUrls = document.RemoteUrls
	service.refreshText()
}

func (service *MessageService) documentBody() string {
	if service.TranslationVisible && service.translatedHtml != "" {
		return service.translatedHtml
	}
	return service.Email.HtmlBody
}

func (MessageService) darkMode() string { return "dark" }

func (service *MessageService) colorMode(mode string) string {
	if mode == "theme" {
		return TextUtils{}.Choose(service.theme == service.darkMode(), service.darkMode(), "light")
	}
	return mode
}

func (service *MessageService) unloadedUrls() []string {
	return slices.DeleteFunc(slices.Clone(service.remoteUrls), func(address string) bool { return service.remoteSources[address] != "" })
}

func (service *MessageService) loadRemoteImages(urls []string) {
	if len(urls) > 0 && !service.RemoteImagesLoading && service.HasEmail {
		service.fetchRemoteImages(urls)
	}
}

func (service *MessageService) fetchRemoteImages(urls []string) {
	service.RemoteImagesLoading = true
	service.RemoteImageError = RemoteImageUtils{}.Failure(service.fetchImages(urls))
	service.refreshDocument()
	service.RemoteImagesLoading = false
}

func (MessageService) remoteBatch() int { return 1 }

func (service *MessageService) fetchImages(urls []string) int {
	failures := 0
	for start := 0; start < len(urls); start += service.remoteBatch() {
		batch := urls[start:min(start+service.remoteBatch(), len(urls))]
		images, failure := service.inboxClient.Images(service.Email.Id, map[string]any{"urls": batch}, service.authorization())
		failures += RemoteImageUtils{}.Failures(images, failure, len(batch))
		RemoteImageUtils{}.Keep(service.remoteSources, images.Images)
	}
	return failures
}

func (service *MessageService) trustSender(sender string) {
	if sender != "" && !service.RemotePreferenceSaving {
		service.saveTrustedSender(sender)
	}
}

func (service *MessageService) saveTrustedSender(sender string) {
	service.RemotePreferenceSaving = true
	saved, failure := service.usersClient.SavePreferences(map[string]any{"trustedImageSenders": append(slices.Clone(service.trustedSenders), sender)}, service.authorization())
	service.senderTrusted(saved, failure == nil, sender)
	service.RemotePreferenceSaving = false
}

func (service *MessageService) senderTrusted(saved ImagePreferences, stored bool, sender string) {
	if !stored {
		service.toast("Could not save the image preference.")
		return
	}
	service.trustedSenders = AddressUtils{}.NormalizedAll(saved.TrustedImageSenders)
	service.ShowRemoteImages()
	service.toast("Images from " + sender + " will load automatically.")
}

func (service *MessageService) rasterAttachments(attachments []EmailAttachment) []EmailAttachment {
	return slices.DeleteFunc(slices.Clone(attachments), func(attachment EmailAttachment) bool {
		return !AttachmentContentUtils{}.IsRasterImage(attachment.ContentType)
	})
}

func (service *MessageService) prepareAttachments(rasters []EmailAttachment) {
	for _, attachment := range rasters {
		content, failure := service.attachmentContent(attachment.Id)
		service.keepPreview(attachment, AttachmentContentUtils{}.PreviewDataUrl(content.ContentType, content.ContentBase64), failure == nil)
	}
	service.applyThumbnails()
}

func (service *MessageService) keepPreview(attachment EmailAttachment, source string, loaded bool) {
	if loaded && source != "" {
		service.thumbnails[attachment.Id] = source
		service.inlineSources = AttachmentContentUtils{}.WithInlineSource(service.inlineSources, attachment.ContentId, source)
	}
}

func (service *MessageService) applyThumbnails() {
	for index := range service.Email.Attachments {
		service.Email.Attachments[index].ThumbnailUrl = TextUtils{}.Choose(service.preferences.DisplayAttachmentPreviews, service.thumbnails[service.Email.Attachments[index].Id], "")
	}
}

func (service *MessageService) attachmentContent(attachmentId string) (AttachmentContent, error) {
	cached, found := service.attachments[attachmentId]
	if found {
		return cached, nil
	}
	return service.fetchAttachment(attachmentId)
}

func (service *MessageService) fetchAttachment(attachmentId string) (AttachmentContent, error) {
	content, failure := service.inboxClient.Attachment(service.Email.Id, attachmentId, service.authorization())
	return service.keepAttachment(attachmentId, content, failure)
}

func (service *MessageService) keepAttachment(attachmentId string, content AttachmentContent, failure error) (AttachmentContent, error) {
	if failure != nil || content.Id != attachmentId || content.ContentBase64 == "" {
		return AttachmentContent{}, fmt.Errorf("Attachment could not be loaded.")
	}
	service.attachments[attachmentId] = content
	return content, nil
}

func (service *MessageService) previewAttachment(attachment EmailAttachment, kind string) {
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
	preview := AttachmentPreviewContent{Kind: kind, Text: text, Url: AttachmentContentUtils{}.PreviewDataUrl(content.ContentType, content.ContentBase64)}
	service.showPreview(preview, AttachmentContentUtils{}.PreviewFailure(kind, preview.Url, failure, textFailure))
}

func (service *MessageService) showPreview(preview AttachmentPreviewContent, failure string) {
	service.AttachmentPreviewError = failure
	if failure == "" {
		service.AttachmentPreviewText = TextUtils{}.Choose(preview.Kind == service.textMode(), preview.Text, "")
		service.AttachmentPreviewUrl = TextUtils{}.Choose(preview.Kind == service.textMode(), "", preview.Url)
	}
}

func (service *MessageService) closeAttachmentPreview() {
	service.AttachmentPreviewOpen = false
	service.AttachmentPreviewLoading = false
	service.AttachmentPreviewKind = ""
	service.AttachmentPreviewUrl = ""
	service.AttachmentPreviewText = ""
	service.AttachmentPreviewError = ""
	service.AttachmentPreview = EmailAttachment{}
}

func (service *MessageService) offerAttachment(content AttachmentContent, failure error, filename string) {
	if failure != nil || !service.HasEmail {
		service.toast("Attachment download failed.")
		return
	}
	service.download("data:"+TextUtils{}.Or(content.ContentType, "application/octet-stream")+";base64,"+content.ContentBase64, filename)
}

func (service *MessageService) acceptDateLabels(values []string, labels []string) {
	for index := range min(len(values), len(labels)) {
		service.dateLabels[values[index]] = labels[index]
	}
}

func (service *MessageService) fromCurrentUser() bool {
	sender := AddressUtils{}.Normalized(service.Email.Sender)
	if strings.ToLower(service.CurrentFolder) == service.sentFolder() || (sender != "" && sender == service.ownAddress) {
		return true
	}
	return false
}

func (MessageService) minimumConfidence() float64 { return 0.3 }

func (service *MessageService) detectionAccepted(messageId string, language string, confidence float64) bool {
	if messageId == service.Email.Id && language != "" && confidence >= service.minimumConfidence() && language != service.TranslationTarget {
		return true
	}
	return false
}

func (service *MessageService) automatic(accepted bool, preference string) bool {
	return accepted && preference == "always"
}

func (service *MessageService) languageDetected(accepted bool, language string, preference string) {
	if accepted {
		service.TranslationSource = language
		service.TranslationPreference = preference
	}
}

func (service *MessageService) translateWhen(automatic bool) {
	if automatic {
		service.startTranslation()
	}
}

func (service *MessageService) startTranslation() {
	service.translate()
}

func (service *MessageService) translate() {
	if service.HasEmail && !service.TranslationBusy {
		service.TranslationBusy = true
		service.TranslationProgress = 0
		service.TranslationError = ""
		service.translationParts = map[string][]string{}
		service.applicationEventPublisher.PublishEvent(TranslationRequested{MessageId: service.Email.Id, Source: service.TranslationSource, Target: service.TranslationTarget}, false)
	}
}

func (service *MessageService) receiveTranslation(part TranslationPart) {
	if part.MessageId == service.Email.Id && part.Count > 0 && part.Index >= 0 && part.Index < part.Count {
		service.keepTranslationPart(part)
	}
}

func (service *MessageService) keepTranslationPart(part TranslationPart) {
	service.translationParts[part.Kind] = TranslationPartUtils{}.With(service.translationParts[part.Kind], part)
	service.completeTranslation(part.Kind == "done", part.Text)
}

func (service *MessageService) completeTranslation(done bool, source string) {
	if done {
		service.finishTranslation(source)
	}
}

func (service *MessageService) finishTranslation(source string) {
	service.translatedHtml = strings.Join(service.translationParts[service.htmlMode()], "")
	service.translatedPlain = strings.Join(service.translationParts[service.textMode()], "")
	service.TranslationSource = TextUtils{}.Or(source, service.TranslationSource)
	service.TranslationBusy = false
	service.TranslationProgress = 0
	service.showTranslation(service.hasTranslatedText())
}

func (service *MessageService) hasTranslatedText() bool {
	return service.translatedHtml != "" || service.translatedPlain != ""
}

func (service *MessageService) translationFailed(current bool, message string) {
	if current {
		service.TranslationError = message
		service.TranslationBusy = false
		service.TranslationProgress = 0
	}
}

func (service *MessageService) showTranslation(visible bool) {
	service.TranslationVisible = visible
	service.applicationEventPublisher.PublishEvent(TranslationVisibilityChanged{MessageId: service.Email.Id, Enabled: visible}, false)
}

func (service *MessageService) refreshTranslation() {
	service.HasTranslation = service.hasTranslatedText()
	service.TranslationOfferVisible = service.offerVisible(service.translationActive(service.HasTranslation), service.translationPointless(service.fromCurrentUser()))
	service.TranslationStatus = service.translationStatus(service.downloadingModel())
	service.VisiblePlainText = service.plainText(MessageConversationUtils{}.SplitQuotedText(service.Email.Body), service.translatedPlainVisible())
}

func (service *MessageService) translationActive(translated bool) bool {
	if service.TranslationBusy || service.TranslationVisible || translated || service.TranslationError != "" {
		return true
	}
	return false
}

func (service *MessageService) translationPointless(fromCurrentUser bool) bool {
	if service.TranslationSource == "" || service.TranslationSource == service.TranslationTarget || fromCurrentUser {
		return true
	}
	return false
}

func (service *MessageService) downloadingModel() bool {
	return service.TranslationProgress > 0 && service.TranslationProgress < 1
}

func (service *MessageService) translatedPlainVisible() bool {
	return service.TranslationVisible && service.translatedPlain != ""
}

func (service *MessageService) offerVisible(active bool, pointless bool) bool {
	switch {
	case active:
		return true
	case pointless:
		return false
	}
	return service.TranslationPreference != "never"
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
