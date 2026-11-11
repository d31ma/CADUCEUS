import (
	"cmp"
	"errors"
	"slices"
	"strings"
)

// @Service
type ConversationService struct {
	DomainId        string
	ConversationId  string
	Title           string
	Section         string
	State           string
	Messages        []MessageView
	NextCursor      string
	HasOlder        bool
	LoadingOlder    bool
	Problem         string
	EditingId       string
	EditBody        string
	DeletingId      string
	DeleteWarning   string
	OriginalForId   string
	OriginalUrl     string
	PickerForId     string
	ReactionChoices []MessageReaction
	// @LocalState
	AllowedSenders map[string][]string
	ShowQuoted     []string
	raw            []Message
	unreadCount    int
	// @Needs
	requests *ConversationRequestService
	// @Needs
	messageList *ConversationMessageService
	// @Needs
	views *ConversationViewService
	// @Needs
	shellAnnounce *ShellAnnounceService
	// @Needs
	clock                     Clock
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (service *ConversationService) Start() {
	service.reset()
	service.applicationEventPublisher.PublishEvent(ConversationViewMounted{Mounted: true}, false)
}

// @OnEvent
// @InBackground
func (service *ConversationService) Opened(opened ConversationOpened) {
	service.openWhenCurrent(opened)
}

// @OnEvent({"replay":true})
// @InBackground
func (service *ConversationService) DomainChanged(changed SessionDomainChanged) {
	service.resetWhenOtherDomain(changed.DomainId)
}

// @OnEvent
// @InBackground
func (service *ConversationService) Live(event RealtimeEvent) {
	service.refreshWhenRelevant(event)
}

func (service *ConversationService) Retry() {
	service.openWhenCurrent(ConversationOpened{
		DomainId:       service.DomainId,
		ConversationId: service.ConversationId,
		Title:          service.Title,
		Section:        service.Section,
		UnreadCount:    service.unreadCount,
	})
}

func (service *ConversationService) LoadOlder() {
	conversationId := service.ConversationId
	domainId := service.DomainId
	service.LoadingOlder = true
	page, failure := service.requests.OlderPage(conversationId, service.NextCursor)
	service.LoadingOlder = false
	service.Problem = service.problemOf(failure, service.loadFailed())
	if failure != nil {
		return
	}
	service.addOlderWhenStill(conversationId, domainId, page)
}

func (service *ConversationService) StartEdit(messageId string) {
	service.EditingId = messageId
	service.EditBody = service.messageList.Find(service.raw, messageId).Body
	service.Problem = ""
	service.rebuild()
}

func (service *ConversationService) SaveEdit(text string) {
	service.EditBody = text
	service.saveWhenFilled()
}

func (service *ConversationService) CancelEdit() {
	service.EditingId = ""
	service.EditBody = ""
	service.rebuild()
}

func (service *ConversationService) AskDelete(messageId string) {
	service.DeletingId = messageId
	service.DeleteWarning = service.messageList.DeleteWarning(service.messageList.Find(service.raw, messageId))
}

func (service *ConversationService) CancelDelete() {
	service.DeletingId = ""
	service.DeleteWarning = ""
}

func (service *ConversationService) ConfirmDelete() {
	failure := service.requests.Delete(service.DeletingId)
	service.CancelDelete()
	service.Problem = service.problemOf(errors.Join(failure, service.refetch()), service.deleteFailed())
}

func (service *ConversationService) ToggleReaction(messageId string, name string) {
	isMine := service.messageList.HasReaction(service.raw, messageId, name)
	failure := service.requests.SetReaction(messageId, name, isMine)
	service.ClosePicker()
	service.Problem = service.problemOf(errors.Join(failure, service.refetch()), service.reactionFailed())
}

func (service *ConversationService) OpenPicker(messageId string) {
	service.PickerForId = messageId
	service.ReactionChoices = service.messageList.ReactionChoices(service.raw, messageId)
}

func (service *ConversationService) ClosePicker() {
	service.PickerForId = ""
	service.ReactionChoices = []MessageReaction{}
}

func (service *ConversationService) AllowImages(address string) {
	service.AllowedSenders = service.messageList.WithSender(service.AllowedSenders, service.DomainId, address)
	service.rebuild()
}

func (service *ConversationService) ToggleQuoted(messageId string) {
	service.ShowQuoted = service.messageList.Toggled(service.ShowQuoted, messageId)
	service.rebuild()
}

func (service *ConversationService) FetchOriginal(messageId string) {
	download, failure := service.requests.Original(messageId)
	service.Problem = service.problemOf(failure, service.originalFailed())
	service.keepOriginal(messageId, download, failure)
}

func (service *ConversationService) OpenThread(messageId string) {
	service.applicationEventPublisher.PublishEvent(ConversationThreadRequested{
		DomainId:       service.DomainId,
		ConversationId: service.ConversationId,
		MessageId:      messageId,
	}, false)
}

func (service *ConversationService) openWhenCurrent(opened ConversationOpened) {
	if opened.DomainId != service.requests.DomainId() {
		return
	}
	service.open(opened)
}

func (service *ConversationService) open(opened ConversationOpened) {
	service.reset()
	service.DomainId = opened.DomainId
	service.ConversationId = opened.ConversationId
	service.Title = opened.Title
	service.Section = opened.Section
	service.unreadCount = opened.UnreadCount
	service.State = service.stateLoading()
	page, failure := service.requests.FirstPage(opened.ConversationId)
	service.showWhenStill(opened, page, failure)
}

func (service *ConversationService) showWhenStill(opened ConversationOpened, page MessagePage, failure error) {
	if !service.still(opened.ConversationId, opened.DomainId) {
		return
	}
	service.show(page, failure)
}

func (service *ConversationService) show(page MessagePage, failure error) {
	service.raw = service.messageList.Combine(page.Items, nil)
	service.NextCursor = page.NextCursor
	service.HasOlder = page.NextCursor != ""
	service.Problem = service.problemOf(failure, service.loadFailed())
	service.State = cmp.Or(service.errorWhen(failure), service.emptyWhen(len(service.raw)), service.stateReady())
	service.rebuild()
	service.readyWhenLoaded(failure)
}

func (service *ConversationService) readyWhenLoaded(failure error) {
	if failure != nil {
		return
	}
	service.announceReady()
}

func (service *ConversationService) announceReady() {
	service.markRead()
	service.applicationEventPublisher.PublishEvent(ConversationReady{
		DomainId:       service.DomainId,
		ConversationId: service.ConversationId,
		Title:          service.Title,
		Section:        service.Section,
		Subject:        service.messageList.Subject(service.raw),
	}, true)
}

func (service *ConversationService) addOlderWhenStill(conversationId string, domainId string, page MessagePage) {
	if !service.still(conversationId, domainId) {
		return
	}
	service.addOlder(page)
}

func (service *ConversationService) addOlder(page MessagePage) {
	service.raw = service.messageList.Combine(service.raw, page.Items)
	service.NextCursor = page.NextCursor
	service.HasOlder = page.NextCursor != ""
	service.rebuild()
}

func (service *ConversationService) saveWhenFilled() {
	if strings.TrimSpace(service.EditBody) == "" {
		return
	}
	service.save()
}

func (service *ConversationService) save() {
	edited, failure := service.requests.Edit(service.EditingId, service.EditBody)
	service.Problem = service.problemOf(failure, service.saveFailed())
	service.applyEdit(edited, failure)
}

func (service *ConversationService) applyEdit(edited Message, failure error) {
	if failure != nil {
		return
	}
	service.raw = service.messageList.Replace(service.raw, edited)
	service.CancelEdit()
}

func (service *ConversationService) keepOriginal(messageId string, download MessageDownload, failure error) {
	if failure != nil {
		service.OriginalForId = ""
		service.OriginalUrl = ""
		return
	}
	service.OriginalForId = messageId
	service.OriginalUrl = download.Url
}

func (service *ConversationService) refetch() error {
	conversationId := service.ConversationId
	domainId := service.DomainId
	page, failure := service.requests.FirstPage(conversationId)
	service.mergeWhenStill(conversationId, domainId, page)
	return failure
}

func (service *ConversationService) mergeWhenStill(conversationId string, domainId string, page MessagePage) {
	if !service.still(conversationId, domainId) {
		return
	}
	service.merge(page)
}

func (service *ConversationService) merge(page MessagePage) {
	service.raw = service.messageList.Combine(page.Items, service.raw)
	service.rebuild()
}

func (service *ConversationService) refreshWhenRelevant(event RealtimeEvent) {
	if !service.isRelevant(event) {
		return
	}
	service.refresh(event)
}

func (service *ConversationService) refresh(event RealtimeEvent) {
	service.Problem = cmp.Or(service.problemOf(service.refetch(), service.loadFailed()), service.Problem)
	service.greetWhenCreated(event)
}

func (service *ConversationService) greetWhenCreated(event RealtimeEvent) {
	if event.Type != service.createdType() {
		return
	}
	service.greet(event.MessageId)
}

func (service *ConversationService) greet(messageId string) {
	announcement := service.messageList.Announcement(service.messageList.Find(service.raw, messageId), service.requests.MyAddress())
	service.shellAnnounce.Message = cmp.Or(announcement, service.shellAnnounce.Message)
	service.markRead()
}

func (service *ConversationService) markRead() {
	service.requests.MarkRead(service.ConversationId, service.messageList.Newest(service.raw))
}

func (service *ConversationService) isRelevant(event RealtimeEvent) bool {
	return service.allTrue(
		event.ConversationId != "",
		event.ConversationId == service.ConversationId,
		service.DomainId == service.requests.DomainId(),
		slices.Contains(service.liveTypes(), event.Type),
	)
}

func (service *ConversationService) still(conversationId string, domainId string) bool {
	return service.allTrue(
		service.ConversationId == conversationId,
		service.DomainId == domainId,
		service.requests.DomainId() == domainId,
	)
}

func (service *ConversationService) rebuild() {
	now, _ := service.clock.NowMs()
	service.Messages = service.views.Views(service.raw, ConversationViewContext{
		MyAddress:      service.requests.MyAddress(),
		DomainId:       service.DomainId,
		Section:        service.Section,
		UnreadCount:    service.unreadCount,
		NowMs:          now,
		AllowedSenders: service.AllowedSenders[service.DomainId],
		ShowQuoted:     service.ShowQuoted,
		EditingId:      service.EditingId,
	})
}

func (service *ConversationService) resetWhenOtherDomain(domainId string) {
	if domainId == service.DomainId {
		return
	}
	service.reset()
}

func (service *ConversationService) reset() {
	service.DomainId = ""
	service.ConversationId = ""
	service.Title = ""
	service.Section = ""
	service.State = service.stateIdle()
	service.Messages = []MessageView{}
	service.NextCursor = ""
	service.HasOlder = false
	service.LoadingOlder = false
	service.Problem = ""
	service.EditingId = ""
	service.EditBody = ""
	service.CancelDelete()
	service.ClosePicker()
	service.OriginalForId = ""
	service.OriginalUrl = ""
	service.ShowQuoted = []string{}
	service.raw = []Message{}
	service.unreadCount = 0
}

func (service *ConversationService) problemOf(failure error, sentence string) string {
	if failure != nil {
		return sentence
	}
	return ""
}

func (service *ConversationService) errorWhen(failure error) string {
	if failure != nil {
		return service.stateError()
	}
	return ""
}

func (service *ConversationService) emptyWhen(count int) string {
	if count == 0 {
		return service.stateEmpty()
	}
	return ""
}

func (service *ConversationService) allTrue(conditions ...bool) bool {
	return !slices.Contains(conditions, false)
}

func (service *ConversationService) liveTypes() []string {
	return []string{service.createdType(), "message.edited", "message.deleted", "reaction.changed"}
}

func (service *ConversationService) createdType() string {
	return "message.created"
}

func (service *ConversationService) stateIdle() string {
	return "idle"
}

func (service *ConversationService) stateLoading() string {
	return "loading"
}

func (service *ConversationService) stateReady() string {
	return "ready"
}

func (service *ConversationService) stateEmpty() string {
	return "empty"
}

func (service *ConversationService) stateError() string {
	return "error"
}

func (service *ConversationService) loadFailed() string {
	return "We could not load this conversation. Check your connection and try again."
}

func (service *ConversationService) saveFailed() string {
	return "We could not save your change. Try again."
}

func (service *ConversationService) deleteFailed() string {
	return "We could not delete the message. Try again."
}

func (service *ConversationService) reactionFailed() string {
	return "We could not update the reaction. Try again."
}

func (service *ConversationService) originalFailed() string {
	return "We could not get the original email. Try again."
}
