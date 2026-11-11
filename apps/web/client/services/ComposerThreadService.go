import "strings"

// @Service
type ComposerThreadService struct {
	Target         SendThreadTarget
	Body           string
	SendToExternal bool
	DomainId       string
	SendLabel      string
	Problem        string
	ProblemText    string
	Announcement   string
	// @Needs
	send *ComposerSendService
	// @Needs
	problems *ComposerProblemService
	// @Needs
	auth                      *ComposerAuthService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent
func (thread *ComposerThreadService) Open(target SendThreadTarget) {
	thread.Target = target
	thread.clear()
	thread.refresh()
}

func (thread *ComposerThreadService) ChooseExternal(checked bool) {
	thread.SendToExternal = checked
	thread.refresh()
}

func (thread *ComposerThreadService) SendWith(body string) {
	thread.Body = body
	thread.Problem = thread.contentProblem(strings.TrimSpace(body))
	thread.deliverWhenClear(thread.request())
	thread.refresh()
}

func (thread *ComposerThreadService) refresh() {
	thread.DomainId = thread.auth.DomainId()
	thread.SendLabel = thread.sendLabel(thread.SendToExternal)
	thread.ProblemText = thread.problems.Text(thread.Problem)
}

func (thread *ComposerThreadService) request() SendRequest {
	return SendRequest{
		ConversationId: thread.Target.ConversationId,
		ThreadRootId:   thread.Target.ThreadRootId,
		Body:           strings.TrimSpace(thread.Body),
		AttachmentIds:  []string{},
		Recipients:     []RecipientChoice{},
		Email:          thread.emailOptions(thread.SendToExternal),
	}
}

func (thread *ComposerThreadService) emailOptions(sendToExternal bool) *EmailOptions {
	if !sendToExternal {
		return nil
	}
	return &EmailOptions{SendToExternal: true}
}

func (thread *ComposerThreadService) contentProblem(body string) string {
	if body != "" {
		return ""
	}
	return "emptyMessage"
}

func (thread *ComposerThreadService) deliverWhenClear(request SendRequest) {
	if thread.Problem != "" {
		return
	}
	thread.deliver(request)
}

func (thread *ComposerThreadService) deliver(request SendRequest) {
	result, failure := thread.send.Send(thread.composerName(), request)
	thread.Problem = thread.problems.OfFailure(failure)
	thread.sentWhenClear(result)
}

func (thread *ComposerThreadService) sentWhenClear(result SendResult) {
	if thread.Problem != "" {
		return
	}
	thread.clear()
	thread.Announcement = "Reply sent."
	thread.applicationEventPublisher.PublishEvent(SendCompleted{ConversationId: result.Message.ConversationId, MessageId: result.Message.MessageId}, false)
}

func (thread *ComposerThreadService) clear() {
	thread.send.Reset(thread.composerName())
	thread.Body = ""
	thread.SendToExternal = false
	thread.Problem = ""
}

func (thread *ComposerThreadService) sendLabel(sendToExternal bool) string {
	labels := map[bool]string{true: "Send reply and email", false: "Send reply"}
	return labels[sendToExternal]
}

func (thread *ComposerThreadService) composerName() string {
	return "thread"
}
