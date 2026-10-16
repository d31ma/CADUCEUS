import (
	"encoding/json"
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
	PrintMessages     string

	emailID     string
	token       string
	preferences MailPreferences
	findIndex   int
	results     []FindResult
	expanded    map[string]bool
	quotes      map[string]bool
}

// @OnStart
func (service *ConversationService) Start() {
	service.preferences = AppearanceUtils{}.DefaultMail()
	service.reset("")
}

// @OnEvent({"replay":true})
func (service *ConversationService) HearSession(changed SessionChanged) {
	service.token = changed.Token
}

// HearStorage follows the conversation order and name preferences.
// @OnEvent({"replay":true})
func (service *ConversationService) HearStorage(stored StoredValues) {
	service.adoptPreferences(stored.Session)
}

// @OnEvent
func (service *ConversationService) HearStorageWritten(written StorageWritten) {
	service.adoptPreferences(written.Values)
}

// HearMessage loads the thread of the message the reader opened.
// @OnEvent({"replay":true})
func (service *ConversationService) HearMessage(opened OpenMessageChanged) {
	FlowUtils{}.When(opened.EmailId != service.emailID, func() { service.load(opened.EmailId) })()
	service.applicationEventPublisher.PublishEvent(ViewSettled{Source: "conversation"}, false)
}

// HearShortcut runs the find-in-conversation commands.
// @OnEvent
func (service *ConversationService) HearShortcut(pressed ShortcutPressed) {
	actions := map[string]func(){"core:find-in-thread": service.OpenFind, "core:find-in-thread-next": service.NextResult, "core:find-in-thread-previous": service.PreviousResult}
	action, found := actions[pressed.Command]
	FlowUtils{}.When(LogicUtils{}.All(found, service.emailID != ""), action)()
}

// @OnEvent
func (service *ConversationService) Settled(_ ViewSettled) {}

func (service *ConversationService) ToggleMessage(messageID string) {
	service.expanded[messageID] = !service.expanded[messageID]
	service.decorate()
}

func (service *ConversationService) ToggleQuote(messageID string) {
	service.quotes[messageID] = !service.quotes[messageID]
	service.decorate()
}

func (service *ConversationService) OpenFind() {
	service.FindOpen = true
	service.expandAll()
	service.applicationEventPublisher.PublishEvent(FindToggled{Open: true}, false)
	service.applicationEventPublisher.PublishEvent(FocusRequested{Selector: "[data-find-thread-input]"}, false)
}

func (service *ConversationService) CloseFind() {
	service.FindOpen = false
	service.FindTerm = ""
	service.search(-1)
	service.applicationEventPublisher.PublishEvent(FindToggled{Open: false}, false)
}

func (service *ConversationService) FindInput(term string) {
	service.FindTerm = term
	service.search(0)
	service.focusMatch()
}

// FindKey: Escape closes the find bar; Enter and Shift+Enter step.
func (service *ConversationService) FindKey(key string, backwards bool) {
	steps := map[bool]func(){true: service.PreviousResult, false: service.NextResult}
	actions := map[string]func(){"Escape": service.CloseFind, "Enter": steps[backwards]}
	action, found := actions[key]
	FlowUtils{}.When(found, action)()
}

func (service *ConversationService) PreviousResult() {
	service.step(-1)
}

func (service *ConversationService) NextResult() {
	service.step(1)
}

func (service *ConversationService) Reply(messageID string) {
	message := service.message(messageID)
	fields := service.thread(message)
	fields.To = ReplyUtils{}.Target(message.ReplyTo, message.Sender)
	fields.Subject = service.replyPrefix() + message.Subject
	fields.Quote = service.quote(message, service.replyMode())
	fields.Body = service.quoted(message, service.replyMode())
	service.compose(fields)
}

func (service *ConversationService) ReplyAll(messageID string) {
	message := service.message(messageID)
	fields := service.thread(message)
	fields.To = strings.Join(TextUtils{}.NonEmpty([]string{ReplyUtils{}.Target(message.ReplyTo, message.Sender), message.Recipient}), ", ")
	fields.Subject = service.replyPrefix() + message.Subject
	fields.Quote = service.quote(message, service.replyMode())
	fields.Body = service.quoted(message, service.replyMode())
	service.compose(fields)
}

func (ConversationService) replyPrefix() string { return "Re: " }

func (ConversationService) replyMode() string { return "reply" }

func (service *ConversationService) Forward(messageID string) {
	message := service.message(messageID)
	mode := "forward"
	service.compose(ComposeFields{Subject: "Fwd: " + message.Subject, Body: service.quoted(message, mode), Quote: service.quote(message, mode), SourceEmailID: message.Id})
}

// DownloadOriginal asks MessageService for the message's .eml.
func (service *ConversationService) DownloadOriginal(messageID string) {
	service.applicationEventPublisher.PublishEvent(MessageSourceRequested{EmailId: messageID}, false)
}

// OpenMessage opens the message on its own page.
func (service *ConversationService) OpenMessage(messageID string) {
	service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Message(messageID), false)
}

func (service *ConversationService) adoptPreferences(values map[string]string) {
	text, found := values["caduceus_mail_preferences"]
	FlowUtils{}.When(found, func() { service.preferences = AppearanceUtils{}.Mail(text) })()
	service.decorate()
}

// message finds a card by id; an unknown id gives an empty message.
func (service *ConversationService) message(messageID string) ConversationMessage {
	index := slices.IndexFunc(service.Messages, func(message ConversationMessage) bool { return message.Id == messageID })
	return append([]ConversationMessage{{References: []string{}, ReplyTo: []string{}}}, service.Messages...)[index+1]
}

func (service *ConversationService) reset(emailID string) {
	service.emailID = emailID
	service.Messages = []ConversationMessage{}
	service.results = []FindResult{}
	service.expanded = map[string]bool{}
	service.quotes = map[string]bool{}
	service.FindOpen = false
	service.FindTerm = ""
	service.findIndex = -1
	service.FindCount = 0
	service.refreshLabels()
}

func (service *ConversationService) load(emailID string) {
	service.reset(emailID)
	service.Loading = true
	threads, failure := service.threadsClient.Conversation("Bearer "+service.token, emailID)
	service.Messages = service.sorted(service.records(append(threads, EmailDetailThread{})[0].Emails, failure == nil))
	service.expanded[emailID] = true
	service.search(-1)
	service.Loading = false
}

func (service *ConversationService) records(emails []EmailDetail, loaded bool) []ConversationMessage {
	messages := []ConversationMessage{}
	for _, value := range emails {
		messages = append(messages, EmailRecordUtils{}.Conversation(value))
	}
	return map[bool][]ConversationMessage{true: messages, false: {}}[loaded]
}

// sorted orders the cards by time, newest last unless the preference says
// otherwise.
func (service *ConversationService) sorted(messages []ConversationMessage) []ConversationMessage {
	descending := service.preferences.ConversationOrder == "descending"
	sort.SliceStable(messages, func(left int, right int) bool {
		return service.before(messages[left].ReceivedAt, messages[right].ReceivedAt) != descending
	})
	return messages
}

func (ConversationService) before(left string, right string) bool {
	leftTime, leftFailure := time.Parse(time.RFC3339Nano, left)
	rightTime, rightFailure := time.Parse(time.RFC3339Nano, right)
	parsed := LogicUtils{}.All(leftFailure == nil, rightFailure == nil)
	return map[bool]bool{true: leftTime.Before(rightTime), false: left < right}[parsed]
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
	FlowUtils{}.When(LogicUtils{}.All(service.FindOpen, service.FindCount > 0), func() {
		service.search(service.findIndex + offset)
		service.focusMatch()
	})()
}

// focusMatch opens the card of the current match and scrolls to it.
func (service *ConversationService) focusMatch() {
	for _, result := range service.currentResult() {
		service.expanded[result.MessageId] = true
		service.decorate()
		service.applicationEventPublisher.PublishEvent(FindMatchFocused{Index: result.Index}, false)
	}
}

func (service *ConversationService) currentResult() []FindResult {
	inside := LogicUtils{}.All(service.findIndex >= 0, service.findIndex < len(service.results))
	return service.results[min(max(service.findIndex, 0), len(service.results)):][:map[bool]int{true: 1}[inside]]
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
	message.VisibleConversationSender = TextUtils{}.Choose(LogicUtils{}.All(service.preferences.DetailedNames, message.HeaderFrom != ""), message.HeaderFrom, message.Sender)
	message.VisibleConversationBody = TextUtils{}.Choose(MessageConversationUtils{}.HasText(parts), parts.Visible, "(No body)")
	message.QuotedConversationBody = parts.Quoted
	message.HasConversationQuote = parts.HasQuoted
	whole := strings.Join(TextUtils{}.NonEmpty([]string{message.VisibleConversationBody, message.QuotedConversationBody}), "\n\n")
	message.ConversationBody = TextUtils{}.Choose(message.QuoteExpanded, whole, message.VisibleConversationBody)
}

func (service *ConversationService) refreshLabels() {
	service.HasConversation = len(service.Messages) > 1
	service.CountLabel = TextUtils{}.Count(len(service.Messages), "message", "messages")
	service.FindProgressLabel = service.progress()
	encoded, _ := json.Marshal(service.Messages)
	service.PrintMessages = string(encoded)
}

func (service *ConversationService) progress() string {
	switch {
	case LogicUtils{}.All(service.findIndex >= 0, service.FindCount > 0):
		return fmt.Sprintf("%d of %d", service.findIndex+1, service.FindCount)
	case service.FindTerm != "":
		return "No matches"
	}
	return ""
}

// thread keeps a reply in the message's RFC thread.
func (ConversationService) thread(message ConversationMessage) ComposeFields {
	parent := message.InternetMessageId
	return ComposeFields{InReplyTo: parent, References: ReplyUtils{}.References(message.References, parent), SourceEmailID: message.Id}
}

func (ConversationService) quoted(message ConversationMessage, mode string) string {
	return QuotedMessageUtils{}.Html(*ConversationService{}.quote(message, mode))
}

// quote captures the actual conversation record before prefill serialization.
func (ConversationService) quote(message ConversationMessage, mode string) *QuotedMessage {
	return &QuotedMessage{Mode: mode, Sender: message.Sender, Subject: message.Subject, DateLabel: message.ReceivedLabel, Body: message.Body}
}

// compose hands the fields to the composer: caduceus_compose_prefill in
// sessionStorage, then /compose.
func (service *ConversationService) compose(fields ComposeFields) {
	service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Stored(fields), false)
	service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Opened(), false)
}
