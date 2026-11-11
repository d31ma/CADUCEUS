import "strings"

// @Service
type ThreadService struct {
	ConversationId     string
	RootId             string
	Root               ThreadRow
	HasRoot            bool
	Replies            []ThreadRow
	Timeline           []ThreadRow
	RepliesLabel       string
	Status             string
	NextCursor         string
	LoadingOlder       bool
	FailedDraft        string
	SendStatus         string
	Pending            ThreadRow
	AlsoToConversation bool
	root               Message
	messages           []Message
	draft              string
	pendingId          string
	pendingBody        string
	pendingAlso        bool
	// @Needs
	shell *ShellService
	// @Needs
	announce *ShellAnnounceService
	// @Needs
	remote *ThreadRemoteService
	// @Needs
	rows *ThreadRowService
}

// @OnEvent
func (thread *ThreadService) Refresh(requested ThreadRefreshRequested) {
	thread.refreshWhenOpen(requested.ConversationId)
}

func (thread *ThreadService) Close() {
	thread.shell.CloseThread()
	thread.reset("", "")
}

func (thread *ThreadService) Reload() {
	thread.load()
}

func (thread *ThreadService) ShowOlder() {
	thread.LoadingOlder = true
	page, failure := thread.remote.Page(ThreadPageRequest{ConversationId: thread.ConversationId, RootId: thread.RootId, Cursor: thread.NextCursor})
	thread.messages = append(thread.messages, page.Items...)
	thread.applyPage(page, failure)
}

func (thread *ThreadService) ReplyWith(text string, alsoToConversation bool) {
	thread.replyUnlessSending(thread.SendStatus == thread.sendSending(), text, alsoToConversation)
}

func (thread *ThreadService) Retry() {
	thread.retryWhenFailed()
}

// @OnEvent
func (thread *ThreadService) DomainChanged(changed SessionDomainChanged) {
	thread.shell.ThreadOpen = false
	thread.reset("", "")
}

// @OnEvent
func (thread *ThreadService) Shown(changed VisibilityChanged) {
	thread.refreshWhenShown(changed.Visible)
}

func (thread *ThreadService) OpenMessage(conversationId string, messageId string) {
	thread.reset(conversationId, messageId)
	thread.shell.ThreadOpen = true
	thread.shell.Pane = "thread"
	thread.load()
}

func (thread *ThreadService) load() {
	thread.Status = "loading"
	root, rootFailure := thread.remote.Message(thread.RootId)
	page, failure := thread.remote.Page(ThreadPageRequest{ConversationId: thread.ConversationId, RootId: thread.RootId})
	thread.root = root
	thread.messages = page.Items
	thread.applyPage(page, thread.firstFailure(rootFailure, failure))
}

func (thread *ThreadService) reset(conversationId string, rootId string) {
	thread.ConversationId = conversationId
	thread.RootId = rootId
	thread.root = Message{}
	thread.Root = ThreadRow{}
	thread.HasRoot = false
	thread.messages = []Message{}
	thread.Replies = []ThreadRow{}
	thread.Timeline = []ThreadRow{}
	thread.RepliesLabel = ""
	thread.Status = ""
	thread.NextCursor = ""
	thread.LoadingOlder = false
	thread.draft = ""
	thread.FailedDraft = ""
	thread.SendStatus = ""
	thread.Pending = ThreadRow{}
	thread.pendingId = ""
	thread.pendingBody = ""
	thread.pendingAlso = false
	thread.AlsoToConversation = false
}

func (thread *ThreadService) refreshWhenOpen(conversationId string) {
	if thread.RootId == "" || conversationId != thread.ConversationId {
		return
	}
	thread.load()
}

func (thread *ThreadService) refreshWhenShown(visible bool) {
	if !visible || thread.RootId == "" {
		return
	}
	thread.load()
}

func (thread *ThreadService) firstFailure(rootFailure error, failure error) error {
	if rootFailure != nil {
		return rootFailure
	}
	return failure
}

func (thread *ThreadService) applyPage(page MessagePage, failure error) {
	thread.Status = thread.chosen(thread.codeOf(failure), thread.loadStates(), "error")
	thread.NextCursor = page.NextCursor
	thread.LoadingOlder = false
	thread.showMessages()
}

func (thread *ThreadService) showMessages() {
	thread.Root = thread.rows.Row(thread.root)
	thread.HasRoot = thread.Root.MessageId != ""
	thread.Replies = thread.rows.Replies(thread.messages)
	thread.Timeline = thread.rows.Timeline(thread.root, thread.messages)
	thread.RepliesLabel = thread.rows.RepliesLabel(thread.root, len(thread.Replies))
}

func (thread *ThreadService) forgetPendingWhenChanged() {
	if strings.TrimSpace(thread.draft) == thread.pendingBody {
		return
	}
	thread.pendingId = ""
	thread.SendStatus = ""
}

func (thread *ThreadService) replyUnlessSending(sending bool, text string, alsoToConversation bool) {
	if sending {
		return
	}
	thread.prepareReply(text, alsoToConversation)
}

func (thread *ThreadService) prepareReply(text string, alsoToConversation bool) {
	thread.draft = text
	thread.AlsoToConversation = alsoToConversation
	thread.forgetPendingWhenChanged()
	thread.sendWhen(thread.sendAllowed())
}

func (thread *ThreadService) sendAllowed() bool {
	return strings.TrimSpace(thread.draft) != ""
}

func (thread *ThreadService) sendWhen(allowed bool) {
	if !allowed {
		return
	}
	thread.startSend()
}

func (thread *ThreadService) startSend() {
	thread.pendingBody = strings.TrimSpace(thread.draft)
	thread.pendingAlso = thread.AlsoToConversation
	thread.pendingId = thread.idForPending()
	thread.sendPending()
}

func (thread *ThreadService) retryWhenFailed() {
	if thread.SendStatus != thread.sendFailed() {
		return
	}
	thread.sendPending()
}

func (thread *ThreadService) idForPending() string {
	if thread.pendingId == "" {
		return thread.remote.NewId()
	}
	return thread.pendingId
}

func (thread *ThreadService) sendPending() {
	thread.SendStatus = thread.sendSending()
	thread.Pending = thread.rows.PendingRow(thread.remote.MyName(), thread.pendingBody)
	result, failure := thread.remote.Send(thread.replyRequest())
	thread.finish(result, failure)
}

func (thread *ThreadService) replyRequest() ThreadReplyRequest {
	return ThreadReplyRequest{
		ClientMessageId:    thread.pendingId,
		ConversationId:     thread.ConversationId,
		ThreadRootId:       thread.RootId,
		AlsoToConversation: thread.pendingAlso,
		Body:               thread.pendingBody,
		AttachmentIds:      []string{},
	}
}

func (thread *ThreadService) finish(result ThreadSendResult, failure error) {
	code := thread.codeOf(failure)
	thread.SendStatus = thread.chosen(code, thread.sendStates(), thread.sendFailed())
	thread.announce.Message = thread.chosen(code, thread.announcements(), "Reply not sent")
	thread.FailedDraft = thread.failedDraftOf(failure)
	thread.keepWhenSent(result.Message, failure)
}

func (thread *ThreadService) failedDraftOf(failure error) string {
	if failure == nil {
		return ""
	}
	return thread.pendingBody
}

func (thread *ThreadService) keepWhenSent(message Message, failure error) {
	if failure != nil {
		return
	}
	thread.messages = append([]Message{message}, thread.messages...)
	thread.draft = ""
	thread.pendingId = ""
	thread.pendingBody = ""
	thread.pendingAlso = false
	thread.AlsoToConversation = false
	thread.Pending = ThreadRow{}
	thread.showMessages()
}

func (thread *ThreadService) codeOf(failure error) int {
	if failure == nil {
		return thread.codeOk()
	}
	return BehaviorErrorStatus(failure)
}

func (thread *ThreadService) codeOk() int {
	return 200
}

func (thread *ThreadService) codeUnauthorized() int {
	return 401
}

func (thread *ThreadService) chosen(code int, choices map[int]string, otherwise string) string {
	choice, found := choices[code]
	if found {
		return choice
	}
	return otherwise
}

func (thread *ThreadService) loadStates() map[int]string {
	return map[int]string{thread.codeOk(): "ready", thread.codeUnauthorized(): thread.stateSession(), 404: "gone"}
}

func (thread *ThreadService) sendStates() map[int]string {
	return map[int]string{thread.codeOk(): "", thread.codeUnauthorized(): thread.stateSession()}
}

func (thread *ThreadService) announcements() map[int]string {
	return map[int]string{thread.codeOk(): "Reply sent"}
}

func (thread *ThreadService) stateSession() string {
	return "session"
}

func (thread *ThreadService) sendSending() string {
	return "sending"
}

func (thread *ThreadService) sendFailed() string {
	return "failed"
}
