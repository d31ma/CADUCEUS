import "strings"

// @Service
type ThreadService struct {
	ConversationId string
	RootId         string
	Root           ThreadRow
	HasRoot        bool
	Replies        []ThreadRow
	RepliesLabel   string
	Status         string
	NextCursor     string
	LoadingOlder   bool
	Draft          string
	CanSend        bool
	SendStatus     string
	Pending        ThreadRow
	root           ThreadMessage
	messages       []ThreadMessage
	pendingId      string
	pendingBody    string
	// @Needs
	shell *ShellService
	// @Needs
	announce *ShellAnnounceService
	// @Needs
	remote *ThreadRemoteService
	// @Needs
	rows                      *ThreadRowService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent
func (thread *ThreadService) Open(requested ThreadRootOpenRequested) {
	thread.begin(requested.Root.ConversationId, thread.rootIdOf(requested.Root))
	thread.keepRoot(requested.Root)
}

// @OnEvent
func (thread *ThreadService) OpenMessage(requested ThreadOpenRequested) {
	thread.begin(requested.ConversationId, requested.MessageId)
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
	thread.Status = thread.statusLoading()
	thread.publishLoad("")
}

func (thread *ThreadService) ShowOlder() {
	thread.LoadingOlder = true
	thread.publishLoad(thread.NextCursor)
}

func (thread *ThreadService) Write(text string) {
	thread.Draft = text
	thread.forgetPendingWhenChanged()
	thread.CanSend = thread.sendAllowed()
}

func (thread *ThreadService) Reply() {
	thread.sendWhen(thread.sendAllowed())
}

func (thread *ThreadService) Retry() {
	thread.retryWhenFailed()
}

// @OnEvent
// @InBackground
func (thread *ThreadService) Load(requested ThreadLoadRequested) {
	page, failure := thread.remote.Page(requested)
	thread.applyWhenCurrent(requested, page, failure)
}

// @OnEvent
// @InBackground
func (thread *ThreadService) Send(requested ThreadReplyRequested) {
	result, failure := thread.remote.Send(requested.Request)
	thread.finishWhenCurrent(requested.Request, result, failure)
}

// @OnEvent
func (thread *ThreadService) Shown(changed VisibilityChanged) {
	thread.refreshWhenShown(changed.Visible)
}

func (thread *ThreadService) begin(conversationId string, rootId string) {
	thread.reset(conversationId, rootId)
	thread.Status = thread.statusLoading()
	thread.shell.ThreadOpen = true
	thread.shell.Pane = thread.paneThread()
	thread.publishLoad("")
}

func (thread *ThreadService) reset(conversationId string, rootId string) {
	thread.ConversationId = conversationId
	thread.RootId = rootId
	thread.root = ThreadMessage{}
	thread.Root = ThreadRow{}
	thread.HasRoot = false
	thread.messages = []ThreadMessage{}
	thread.Replies = []ThreadRow{}
	thread.RepliesLabel = ""
	thread.Status = ""
	thread.NextCursor = ""
	thread.LoadingOlder = false
	thread.Draft = ""
	thread.CanSend = false
	thread.SendStatus = ""
	thread.Pending = ThreadRow{}
	thread.pendingId = ""
	thread.pendingBody = ""
}

func (thread *ThreadService) publishLoad(cursor string) {
	requested := ThreadLoadRequested{ConversationId: thread.ConversationId, RootId: thread.RootId, Cursor: cursor}
	thread.applicationEventPublisher.PublishEvent(requested, false)
}

func (thread *ThreadService) rootIdOf(message ThreadMessage) string {
	if message.ThreadRootId == "" {
		return message.MessageId
	}
	return message.ThreadRootId
}

func (thread *ThreadService) keepRoot(message ThreadMessage) {
	if message.ThreadRootId != "" {
		return
	}
	thread.root = message
	thread.showMessages()
}

func (thread *ThreadService) refreshWhenOpen(conversationId string) {
	if thread.RootId == "" || conversationId != thread.ConversationId {
		return
	}
	thread.publishLoad("")
}

func (thread *ThreadService) refreshWhenShown(visible bool) {
	if !visible || thread.RootId == "" {
		return
	}
	thread.publishLoad("")
}

func (thread *ThreadService) applyWhenCurrent(requested ThreadLoadRequested, page ThreadMessagePage, failure error) {
	if requested.RootId != thread.RootId || requested.ConversationId != thread.ConversationId {
		return
	}
	thread.apply(requested.Cursor, page, failure)
}

func (thread *ThreadService) apply(cursor string, page ThreadMessagePage, failure error) {
	thread.Status = thread.chosen(thread.codeOf(failure), thread.loadStates(), thread.statusError())
	thread.messages = thread.messagesWith(cursor, page.Items)
	thread.NextCursor = page.NextCursor
	thread.LoadingOlder = false
	thread.showMessages()
}

func (thread *ThreadService) messagesWith(cursor string, items []ThreadMessage) []ThreadMessage {
	if cursor == "" {
		return items
	}
	return append(thread.messages, items...)
}

func (thread *ThreadService) showMessages() {
	thread.root = thread.rows.Root(thread.messages, thread.RootId, thread.root)
	thread.Root = thread.rows.Row(thread.root)
	thread.HasRoot = thread.root.MessageId != ""
	thread.Replies = thread.rows.Replies(thread.messages, thread.RootId)
	thread.RepliesLabel = thread.rows.RepliesLabel(max(len(thread.Replies), thread.root.ReplyCount))
}

func (thread *ThreadService) forgetPendingWhenChanged() {
	if strings.TrimSpace(thread.Draft) == thread.pendingBody {
		return
	}
	thread.pendingId = ""
	thread.SendStatus = ""
}

func (thread *ThreadService) sendAllowed() bool {
	return strings.TrimSpace(thread.Draft) != "" && thread.SendStatus != thread.sendSending()
}

func (thread *ThreadService) sendWhen(allowed bool) {
	if !allowed {
		return
	}
	thread.startSend()
}

func (thread *ThreadService) startSend() {
	thread.pendingBody = strings.TrimSpace(thread.Draft)
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
	thread.CanSend = false
	thread.Pending = thread.rows.Row(ThreadMessage{Author: ThreadAuthor{DisplayName: thread.remote.MyName()}, Body: thread.pendingBody})
	thread.applicationEventPublisher.PublishEvent(ThreadReplyRequested{Request: thread.replyRequest()}, false)
}

func (thread *ThreadService) replyRequest() ThreadReplyRequest {
	return ThreadReplyRequest{
		ClientMessageId: thread.pendingId,
		ConversationId:  thread.ConversationId,
		ThreadRootId:    thread.RootId,
		Body:            thread.pendingBody,
		AttachmentIds:   []string{},
	}
}

func (thread *ThreadService) finishWhenCurrent(request ThreadReplyRequest, result ThreadSendResult, failure error) {
	if request.ThreadRootId != thread.RootId || request.ClientMessageId != thread.pendingId {
		return
	}
	thread.finish(result, failure)
}

func (thread *ThreadService) finish(result ThreadSendResult, failure error) {
	code := thread.codeOf(failure)
	thread.SendStatus = thread.chosen(code, thread.sendStates(), thread.sendFailed())
	thread.announce.Message = thread.chosen(code, thread.announcements(), thread.notSent())
	thread.keepWhenSent(result.Message, failure)
	thread.CanSend = thread.sendAllowed()
}

func (thread *ThreadService) keepWhenSent(message ThreadMessage, failure error) {
	if failure != nil {
		return
	}
	thread.messages = append([]ThreadMessage{message}, thread.messages...)
	thread.Draft = ""
	thread.pendingId = ""
	thread.pendingBody = ""
	thread.Pending = ThreadRow{}
	thread.showMessages()
}

func (thread *ThreadService) codeOf(failure error) int {
	if failure == nil {
		return thread.codeOk()
	}
	return BehaviorErrorStatus(failure)
}

func (thread *ThreadService) chosen(code int, choices map[int]string, otherwise string) string {
	choice, found := choices[code]
	if found {
		return choice
	}
	return otherwise
}

func (thread *ThreadService) loadStates() map[int]string {
	return map[int]string{thread.codeOk(): thread.statusReady(), thread.codeUnauthorized(): thread.stateSession()}
}

func (thread *ThreadService) sendStates() map[int]string {
	return map[int]string{thread.codeOk(): "", thread.codeUnauthorized(): thread.stateSession()}
}

func (thread *ThreadService) announcements() map[int]string {
	return map[int]string{thread.codeOk(): "Reply sent"}
}

func (thread *ThreadService) notSent() string {
	return "Reply not sent"
}

func (thread *ThreadService) codeOk() int {
	return 200
}

func (thread *ThreadService) codeUnauthorized() int {
	return 401
}

func (thread *ThreadService) statusLoading() string {
	return "loading"
}

func (thread *ThreadService) statusReady() string {
	return "ready"
}

func (thread *ThreadService) statusError() string {
	return "error"
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

func (thread *ThreadService) paneThread() string {
	return "thread"
}
