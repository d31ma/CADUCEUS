import "strings"

// @Service
type ThreadService struct {
	ConversationId     string
	RootId             string
	Root               ThreadRow
	HasRoot            bool
	Replies            []ThreadRow
	RepliesLabel       string
	Status             string
	NextCursor         string
	LoadingOlder       bool
	Draft              string
	CanSend            bool
	SendStatus         string
	Pending            ThreadRow
	AlsoToConversation bool
	root               ThreadMessage
	messages           []ThreadMessage
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
	rows                      *ThreadRowService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent
func (thread *ThreadService) Open(requested ThreadRootOpenRequested) {
	thread.begin(thread.rows.ConversationOf(requested.Root), thread.rows.RootIdOf(requested.Root))
	thread.root = thread.rows.KnownRoot(requested.Root)
	thread.showMessages()
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
	thread.publishLoad()
}

func (thread *ThreadService) ShowOlder() {
	thread.LoadingOlder = true
	thread.applicationEventPublisher.PublishEvent(ThreadOlderRequested{ConversationId: thread.ConversationId, RootId: thread.RootId, Cursor: thread.NextCursor}, false)
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
	root, rootFailure := thread.remote.Message(requested.RootId)
	page, failure := thread.remote.Page(ThreadPageRequest{ConversationId: requested.ConversationId, RootId: requested.RootId})
	thread.applyWhenCurrent(requested, root, page, thread.firstFailure(rootFailure, failure))
}

// @OnEvent
// @InBackground
func (thread *ThreadService) LoadOlder(requested ThreadOlderRequested) {
	page, failure := thread.remote.Page(ThreadPageRequest(requested))
	thread.applyOlderWhenCurrent(requested, page, failure)
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
	thread.publishLoad()
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
	thread.pendingAlso = false
	thread.AlsoToConversation = false
}

func (thread *ThreadService) publishLoad() {
	requested := ThreadLoadRequested{ConversationId: thread.ConversationId, RootId: thread.RootId}
	thread.applicationEventPublisher.PublishEvent(requested, false)
}

func (thread *ThreadService) refreshWhenOpen(conversationId string) {
	if thread.RootId == "" || conversationId != thread.ConversationId {
		return
	}
	thread.publishLoad()
}

func (thread *ThreadService) refreshWhenShown(visible bool) {
	if !visible || thread.RootId == "" {
		return
	}
	thread.publishLoad()
}

func (thread *ThreadService) firstFailure(rootFailure error, failure error) error {
	if rootFailure != nil {
		return rootFailure
	}
	return failure
}

func (thread *ThreadService) applyWhenCurrent(requested ThreadLoadRequested, root ThreadMessage, page ThreadMessagePage, failure error) {
	if requested.RootId != thread.RootId || requested.ConversationId != thread.ConversationId {
		return
	}
	thread.root = root
	thread.messages = page.Items
	thread.applyPage(page, failure)
}

func (thread *ThreadService) applyOlderWhenCurrent(requested ThreadOlderRequested, page ThreadMessagePage, failure error) {
	if requested.RootId != thread.RootId || requested.ConversationId != thread.ConversationId {
		return
	}
	thread.messages = append(thread.messages, page.Items...)
	thread.applyPage(page, failure)
}

func (thread *ThreadService) applyPage(page ThreadMessagePage, failure error) {
	thread.Status = thread.chosen(thread.codeOf(failure), thread.loadStates(), thread.statusError())
	thread.NextCursor = page.NextCursor
	thread.LoadingOlder = false
	thread.showMessages()
}

func (thread *ThreadService) showMessages() {
	thread.Root = thread.rows.Row(thread.root)
	thread.HasRoot = thread.Root.MessageId != ""
	thread.Replies = thread.rows.Replies(thread.messages)
	thread.RepliesLabel = thread.rows.RepliesLabel(thread.root, len(thread.Replies))
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
	thread.CanSend = false
	thread.Pending = thread.rows.PendingRow(thread.remote.MyName(), thread.pendingBody)
	thread.applicationEventPublisher.PublishEvent(ThreadReplyRequested{Request: thread.replyRequest()}, false)
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

func (thread *ThreadService) chosen(code int, choices map[int]string, otherwise string) string {
	choice, found := choices[code]
	if found {
		return choice
	}
	return otherwise
}

func (thread *ThreadService) loadStates() map[int]string {
	return map[int]string{thread.codeOk(): thread.statusReady(), thread.codeUnauthorized(): thread.stateSession(), thread.codeNotFound(): thread.statusGone()}
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

func (thread *ThreadService) codeNotFound() int {
	return 404
}

func (thread *ThreadService) statusLoading() string {
	return "loading"
}

func (thread *ThreadService) statusReady() string {
	return "ready"
}

func (thread *ThreadService) statusGone() string {
	return "gone"
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
