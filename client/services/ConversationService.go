import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// ConversationService shows every authorized message of the open message's
// RFC thread as reading cards, with per-card quoted text, replies and
// find-in-conversation.
//
// @Service
type ConversationService struct {
	// @Needs
	threadsClient             *ThreadsClient
	applicationEventPublisher ApplicationEventPublisher

	Messages          []ConversationMessage
	Loading           bool
	HasConversation   bool
	CountLabel        string
	FindOpen          bool
	FindTerm          string
	FindCount         int
	FindProgressLabel string
	ComposePrefill    ComposePrefill

	emailId     string
	token       string
	preferences MailPreferences
	findIndex   int
	results     []FindResult
	expanded    map[string]bool
	quotes      map[string]bool
	dateLabels  map[string]string
}

// @OnStart
func (service *ConversationService) Start() {
	service.preferences = MailPreferencesUtils{}.Defaults()
	service.reset("")
}

// @OnEvent({"replay":true})
func (service *ConversationService) SessionChanged(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *ConversationService) PreferencesChanged(changed MailPreferencesChanged) {
	service.preferences = MailPreferencesUtils{}.Normalize(changed)
}

func (ConversationService) owner() string { return "conversation" }

// @OnEvent({"replay":true})
func (service *ConversationService) MessageOpened(opened OpenMessageChanged) error {
	failure := service.open(opened.EmailId)
	service.applicationEventPublisher.PublishEvent(ReaderChanged{Reason: service.owner()}, false)
	return failure
}

// @OnEvent
func (service *ConversationService) Shortcut(shortcut ShortcutTriggered) {
	actions := map[string]func(){"core:find-in-thread": service.OpenFind, "core:find-in-thread-next": service.NextResult, "core:find-in-thread-previous": service.PreviousResult}
	service.run(actions[shortcut.Name])
	service.applicationEventPublisher.PublishEvent(ReaderChanged{Reason: service.owner()}, false)
}

func (service *ConversationService) ToggleMessage(messageId string) {
	service.expanded[messageId] = !service.expanded[messageId]
	service.decorate()
}

func (service *ConversationService) ToggleQuote(messageId string) {
	service.quotes[messageId] = !service.quotes[messageId]
	service.decorate()
}

func (service *ConversationService) OpenFind() {
	service.FindOpen = true
	service.expandAll()
	service.applicationEventPublisher.PublishEvent(FindToggled{Open: true}, false)
	service.applicationEventPublisher.PublishEvent(FindMatchFocused{Index: -1}, false)
}

func (service *ConversationService) CloseFind() {
	service.FindOpen = false
	service.FindTerm = ""
	service.findIndex = -1
	service.search(-1)
	service.applicationEventPublisher.PublishEvent(FindToggled{Open: false}, false)
}

func (service *ConversationService) FindInput(term string) {
	service.FindTerm = term
	service.search(0)
	service.focusMatch()
}

func (service *ConversationService) FindKey(key string, backwards bool) {
	actions := map[string]func(){"Escape": service.CloseFind, "Enter": service.directionStep(backwards)}
	service.run(actions[key])
}

func (service *ConversationService) PreviousResult() {
	service.step(-1)
}

func (service *ConversationService) NextResult() {
	service.step(1)
}

func (ConversationService) replyPrefix() string { return "Re: " }

func (ConversationService) replyMode() string { return "reply" }

func (service *ConversationService) Reply(messageId string) {
	message := service.message(messageId)
	prefill := service.thread(message)
	prefill.To = ReplyUtils{}.Target(message.ReplyTo, message.Sender)
	prefill.Subject = service.replyPrefix() + message.Subject
	prefill.Body = service.quoted(message, service.replyMode())
	service.compose(prefill)
}

func (service *ConversationService) ReplyAll(messageId string) {
	message := service.message(messageId)
	prefill := service.thread(message)
	prefill.To = strings.Join(TextUtils{}.NonEmpty([]string{ReplyUtils{}.Target(message.ReplyTo, message.Sender), message.Recipient}), ", ")
	prefill.Subject = service.replyPrefix() + message.Subject
	prefill.Body = service.quoted(message, service.replyMode())
	service.compose(prefill)
}

func (service *ConversationService) Forward(messageId string) {
	message := service.message(messageId)
	service.compose(ComposePrefill{Subject: "Fwd: " + message.Subject, Body: service.quoted(message, "forward"), SourceEmailId: message.Id, References: []string{}})
}

func (service *ConversationService) DownloadOriginal(messageId string) {
	service.applicationEventPublisher.PublishEvent(MessageSourceRequested{EmailId: messageId}, false)
}

func (service *ConversationService) OpenMessage(messageId string) {
	service.applicationEventPublisher.PublishEvent(EmailOpenRequested{EmailId: messageId}, false)
}

func (service *ConversationService) message(messageId string) ConversationMessage {
	index := slices.IndexFunc(service.Messages, func(message ConversationMessage) bool { return message.Id == messageId })
	return service.messageAt(index)
}

func (service *ConversationService) messageAt(index int) ConversationMessage {
	if index < 0 {
		return ConversationMessage{References: []string{}, ReplyTo: []string{}}
	}
	return service.Messages[index]
}

func (service *ConversationService) AcceptDateLabels(values []string, labels []string) {
	service.acceptDateLabels(values, labels)
}

func (service *ConversationService) acceptDateLabels(values []string, labels []string) {
	for index := range min(len(values), len(labels)) {
		service.dateLabels[values[index]] = labels[index]
	}
}

func (service *ConversationService) reset(emailId string) {
	service.emailId = emailId
	service.Messages = []ConversationMessage{}
	service.results = []FindResult{}
	service.expanded = map[string]bool{}
	service.quotes = map[string]bool{}
	service.dateLabels = map[string]string{}
	service.FindOpen = false
	service.FindTerm = ""
	service.findIndex = -1
	service.FindCount = 0
	service.refreshLabels()
}

func (service *ConversationService) open(emailId string) error {
	if emailId == service.emailId {
		return nil
	}
	return service.load(emailId)
}

func (service *ConversationService) load(emailId string) error {
	service.reset(emailId)
	service.Loading = true
	threads, failure := service.threadsClient.Find(emailId, "Bearer "+service.token)
	service.Messages = service.sorted(service.records(threads, failure == nil))
	service.expanded[emailId] = true
	service.applicationEventPublisher.PublishEvent(DateLabelsRequested{Owner: service.owner(), Values: service.receivedTimes()}, false)
	service.search(-1)
	service.Loading = false
	return nil
}

func (service *ConversationService) records(threads []ThreadRecord, loaded bool) []ConversationMessage {
	if !loaded || len(threads) == 0 {
		return []ConversationMessage{}
	}
	return service.messages(threads[0].Emails)
}

func (service *ConversationService) messages(emails []BehaviorValue) []ConversationMessage {
	messages := []ConversationMessage{}
	for _, value := range emails {
		messages = append(messages, EmailRecordUtils{}.Conversation(RecordUtils{}.Object(value)))
	}
	return messages
}

func (service *ConversationService) sorted(messages []ConversationMessage) []ConversationMessage {
	descending := service.preferences.ConversationOrder == "descending"
	sort.SliceStable(messages, func(left int, right int) bool {
		return service.before(messages[left].ReceivedAt, messages[right].ReceivedAt) != descending
	})
	return messages
}

func (service *ConversationService) before(left string, right string) bool {
	leftTime, leftFailure := time.Parse(time.RFC3339Nano, left)
	rightTime, rightFailure := time.Parse(time.RFC3339Nano, right)
	if leftFailure != nil || rightFailure != nil {
		return left < right
	}
	return leftTime.Before(rightTime)
}

func (service *ConversationService) receivedTimes() []string {
	values := []string{}
	for _, message := range service.Messages {
		values = append(values, message.ReceivedAt)
	}
	return values
}

func (service *ConversationService) expandAll() {
	for _, message := range service.Messages {
		service.expanded[message.Id] = true
	}
	service.decorate()
}

func (service *ConversationService) search(requested int) {
	found := MessageConversationUtils{}.SearchConversation(service.Messages, service.FindTerm, requested)
	service.Messages = found.Messages
	service.results = found.Results
	service.findIndex = found.Index
	service.FindCount = found.Count
	service.decorate()
}

func (service *ConversationService) step(offset int) {
	if service.FindOpen && service.FindCount > 0 {
		service.move(offset)
	}
}

func (service *ConversationService) move(offset int) {
	service.search(service.findIndex + offset)
	service.focusMatch()
}

func (service *ConversationService) directionStep(backwards bool) func() {
	steps := map[bool]func(){true: service.PreviousResult, false: service.NextResult}
	return steps[backwards]
}

func (service *ConversationService) run(action func()) {
	if action != nil {
		action()
	}
}

func (service *ConversationService) focusMatch() {
	for _, result := range service.currentResult() {
		service.expanded[result.MessageId] = true
		service.decorate()
		service.applicationEventPublisher.PublishEvent(FindMatchFocused{Index: result.Index}, false)
	}
}

func (service *ConversationService) currentResult() []FindResult {
	if service.findIndex < 0 || service.findIndex >= len(service.results) {
		return []FindResult{}
	}
	return []FindResult{service.results[service.findIndex]}
}

func (service *ConversationService) decorate() {
	for index := range service.Messages {
		service.decorateMessage(&service.Messages[index])
	}
	service.refreshLabels()
}

func (service *ConversationService) decorateMessage(message *ConversationMessage) {
	parts := MessageConversationUtils{}.SplitQuotedText(message.Body)
	message.Expanded = service.expanded[message.Id]
	message.QuoteExpanded = service.quotes[message.Id]
	message.VisibleConversationSender = service.sender(message.HeaderFrom, message.Sender)
	message.VisibleConversationBody = TextUtils{}.Choose(MessageConversationUtils{}.HasText(parts), parts.Visible, "(No body)")
	message.QuotedConversationBody = parts.Quoted
	message.HasConversationQuote = parts.HasQuoted
	message.ConversationBody = service.body(message.QuoteExpanded, []string{message.VisibleConversationBody, message.QuotedConversationBody})
}

func (service *ConversationService) sender(headerFrom string, sender string) string {
	if service.preferences.DetailedNames && headerFrom != "" {
		return headerFrom
	}
	return sender
}

func (service *ConversationService) body(quoteExpanded bool, parts []string) string {
	if quoteExpanded {
		return strings.Join(TextUtils{}.NonEmpty(parts), "\n\n")
	}
	return parts[0]
}

func (service *ConversationService) refreshLabels() {
	service.HasConversation = len(service.Messages) > 1
	service.CountLabel = TextUtils{}.Count(len(service.Messages), "message", "messages")
	service.FindProgressLabel = service.progress()
}

func (service *ConversationService) progress() string {
	if service.findIndex >= 0 && service.FindCount > 0 {
		return fmt.Sprintf("%d of %d", service.findIndex+1, service.FindCount)
	}
	return TextUtils{}.Choose(service.FindTerm != "", "No matches", "")
}

func (service *ConversationService) thread(message ConversationMessage) ComposePrefill {
	if message.InternetMessageId == "" {
		return ComposePrefill{SourceEmailId: message.Id, References: []string{}}
	}
	return ComposePrefill{InReplyTo: message.InternetMessageId, References: ReplyUtils{}.References(message.References, message.InternetMessageId), SourceEmailId: message.Id}
}

func (service *ConversationService) quoted(message ConversationMessage, mode string) string {
	return QuotedMessageUtils{}.Html(QuotedMessage{Mode: mode, Sender: message.Sender, Subject: message.Subject, DateLabel: service.dateLabels[message.ReceivedAt], Body: message.Body})
}

func (service *ConversationService) compose(prefill ComposePrefill) {
	service.ComposePrefill = prefill
	service.applicationEventPublisher.PublishEvent(ComposeRequested{Owner: service.owner()}, false)
}
