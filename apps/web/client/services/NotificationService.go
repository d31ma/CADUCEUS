// @Service
type NotificationService struct {
	Status        string
	Tab           string
	TabId         string
	Tabs          []NotificationTab
	Days          []NotificationDay
	Empty         bool
	UnreadCount   int
	NextCursor    string
	HasMore       bool
	ActionProblem string
	items         []Notification
	seen          map[string]bool
	known         bool
	// @Needs
	api *NotificationApiService
	// @Needs
	rows *NotificationRowService
	// @Needs
	push *NotificationPushService
	// @Needs
	pageVisibility            PageVisibility
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (service *NotificationService) Start() {
	service.Status = service.statusLoading()
	service.Tab = service.rows.AllTab()
	service.show()
	service.applicationEventPublisher.PublishEvent(NotificationLoadRequested{}, false)
}

// @OnEvent
// @InBackground
func (service *NotificationService) Load(requested NotificationLoadRequested) {
	page, problem := service.api.Page("")
	visible, failure := service.pageVisibility.Visible()
	service.push.Announce(service.rows.Fresh(page.Items, service.seen, service.known), service.hidden(visible, failure))
	service.seen = service.rows.Seen(page.Items, service.seen)
	service.known = service.knownAfter(problem)
	service.items = page.Items
	service.NextCursor = page.NextCursor
	service.Status = service.statusOf(problem)
	service.show()
}

// @OnEvent
// @InBackground
func (service *NotificationService) DomainChanged(changed SessionDomainChanged) {
	service.seen = nil
	service.known = false
	service.Tab = service.rows.AllTab()
	service.Status = service.statusLoading()
	service.Load(NotificationLoadRequested{})
}

// @OnEvent
// @InBackground
func (service *NotificationService) Shown(changed VisibilityChanged) {
	service.loadWhenVisible(changed.Visible)
}

func (service *NotificationService) Retry() {
	service.Status = service.statusLoading()
	service.applicationEventPublisher.PublishEvent(NotificationLoadRequested{}, false)
}

func (service *NotificationService) LoadMore() {
	page, problem := service.api.Page(service.NextCursor)
	service.items = append(service.items, page.Items...)
	service.seen = service.rows.Seen(page.Items, service.seen)
	service.NextCursor = service.cursorAfter(page, problem)
	service.ActionProblem = problem
	service.show()
}

func (service *NotificationService) ChooseTab(name string) {
	service.Tab = name
	service.show()
}

func (service *NotificationService) Open(row NotificationRow) {
	service.ActionProblem = service.markWhenUnread(row)
	service.applicationEventPublisher.PublishEvent(NotificationOpenRequested{ConversationId: row.ConversationId, MessageId: row.MessageId}, false)
	service.show()
}

func (service *NotificationService) MarkRead(notificationId string) {
	service.ActionProblem = service.markRead(notificationId)
	service.show()
}

func (service *NotificationService) MarkAllRead() {
	service.ActionProblem = service.markEach(service.rows.UnreadIds(service.items, service.markAllLimit()))
	service.show()
}

func (service *NotificationService) show() {
	service.TabId = service.rows.TabId(service.Tab)
	service.Tabs = service.rows.Tabs(service.items, service.Tab)
	service.Days = service.rows.Days(service.items, service.Tab)
	service.Empty = len(service.Days) == 0
	service.UnreadCount = service.rows.Count(service.items, service.rows.AllTab())
	service.HasMore = service.NextCursor != ""
}

func (service *NotificationService) loadWhenVisible(visible bool) {
	if !visible {
		return
	}
	service.Load(NotificationLoadRequested{})
}

func (service *NotificationService) markWhenUnread(row NotificationRow) string {
	if !row.Unread {
		return ""
	}
	return service.markRead(row.NotificationId)
}

func (service *NotificationService) markEach(notificationIds []string) string {
	problem := ""
	for _, notificationId := range notificationIds {
		problem = service.firstProblem(problem, service.markRead(notificationId))
	}
	return problem
}

func (service *NotificationService) markRead(notificationId string) string {
	problem := service.api.MarkRead(notificationId)
	service.items = service.rows.Read(service.items, notificationId, problem)
	return problem
}

func (service *NotificationService) firstProblem(first string, next string) string {
	if first != "" {
		return first
	}
	return next
}

func (service *NotificationService) cursorAfter(page NotificationPage, problem string) string {
	if problem != "" {
		return service.NextCursor
	}
	return page.NextCursor
}

func (service *NotificationService) statusOf(problem string) string {
	if problem != "" {
		return problem
	}
	return "ready"
}

func (service *NotificationService) knownAfter(problem string) bool {
	return service.known || problem == ""
}

func (service *NotificationService) hidden(visible bool, failure error) bool {
	return failure == nil && !visible
}

func (service *NotificationService) statusLoading() string {
	return "loading"
}

func (service *NotificationService) markAllLimit() int {
	return 100
}
