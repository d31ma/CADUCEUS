import (
	"maps"
	"slices"
)

// @Service
type NotificationRowService struct {
	// @Needs
	times *NotificationTimeService
	// @Needs
	kinds *NotificationKindService
}

func (service *NotificationRowService) AllTab() string {
	return service.kinds.AllTab()
}

func (service *NotificationRowService) TabId(name string) string {
	return "activity-tab-" + name
}

func (service *NotificationRowService) Tabs(items []Notification, selected string) []NotificationTab {
	return service.tabsOf(items, selected)
}

func (service *NotificationRowService) Days(items []Notification, tab string) []NotificationDay {
	return service.grouped(service.rowsIn(items, tab))
}

func (service *NotificationRowService) Count(items []Notification, tab string) int {
	return service.countIn(items, tab)
}

func (service *NotificationRowService) UnreadIds(items []Notification, limit int) []string {
	return service.unreadIdsOf(items, limit)
}

func (service *NotificationRowService) Fresh(items []Notification, seen map[string]bool, known bool) []Notification {
	return service.freshWhenKnown(items, seen, known)
}

func (service *NotificationRowService) Seen(items []Notification, seen map[string]bool) map[string]bool {
	merged := map[string]bool{}
	maps.Copy(merged, seen)
	return service.withIds(merged, items)
}

func (service *NotificationRowService) Read(items []Notification, notificationId string, problem string) []Notification {
	return service.readWhenClear(items, notificationId, problem)
}

func (service *NotificationRowService) tabsOf(items []Notification, selected string) []NotificationTab {
	tabs := []NotificationTab{}
	for _, name := range service.kinds.TabNames() {
		tabs = append(tabs, service.tabOf(items, name, selected))
	}
	return tabs
}

func (service *NotificationRowService) tabOf(items []Notification, name string, selected string) NotificationTab {
	count := service.countIn(items, name)
	return NotificationTab{
		Name:        name,
		Id:          service.TabId(name),
		Label:       service.kinds.TabLabel(name),
		UnreadCount: count,
		HasUnread:   count > 0,
		Selected:    name == selected,
	}
}

func (service *NotificationRowService) countIn(items []Notification, tab string) int {
	count := 0
	for _, item := range items {
		count += service.oneWhenUnreadIn(item, tab)
	}
	return count
}

func (service *NotificationRowService) oneWhenUnreadIn(item Notification, tab string) int {
	if item.ReadAt != "" || !service.inTab(item, tab) {
		return 0
	}
	return 1
}

func (service *NotificationRowService) inTab(item Notification, tab string) bool {
	return slices.Contains([]string{service.kinds.AllTab(), item.Kind}, tab)
}

func (service *NotificationRowService) rowsIn(items []Notification, tab string) []NotificationRow {
	rows := []NotificationRow{}
	for _, item := range items {
		rows = service.addWhenInTab(rows, item, tab)
	}
	return rows
}

func (service *NotificationRowService) addWhenInTab(rows []NotificationRow, item Notification, tab string) []NotificationRow {
	if !service.inTab(item, tab) {
		return rows
	}
	return append(rows, service.rowOf(item))
}

func (service *NotificationRowService) rowOf(item Notification) NotificationRow {
	title := service.kinds.Text(item.Kind)
	clockText := service.times.ClockText(item.CreatedAt)
	return NotificationRow{
		NotificationId: item.NotificationId,
		Kind:           item.Kind,
		ConversationId: item.ConversationId,
		MessageId:      item.MessageId,
		Title:          title,
		Time:           clockText,
		Stamp:          item.CreatedAt,
		Day:            service.times.DayText(item.CreatedAt),
		Unread:         item.ReadAt == "",
		MarkLabel:      "Mark as read: " + title + ", " + clockText,
	}
}

func (service *NotificationRowService) grouped(rows []NotificationRow) []NotificationDay {
	days := []NotificationDay{}
	for _, row := range rows {
		days = service.addToDay(days, row)
	}
	return days
}

func (service *NotificationRowService) addToDay(days []NotificationDay, row NotificationRow) []NotificationDay {
	last := len(days) - 1
	if last >= 0 && days[last].Label == row.Day {
		days[last].Rows = append(days[last].Rows, row)
		return days
	}
	return append(days, NotificationDay{Label: row.Day, Rows: []NotificationRow{row}})
}

func (service *NotificationRowService) unreadIdsOf(items []Notification, limit int) []string {
	ids := []string{}
	for _, item := range items {
		ids = service.addWhenUnread(ids, item, limit)
	}
	return ids
}

func (service *NotificationRowService) addWhenUnread(ids []string, item Notification, limit int) []string {
	if item.ReadAt != "" || len(ids) >= limit {
		return ids
	}
	return append(ids, item.NotificationId)
}

func (service *NotificationRowService) freshWhenKnown(items []Notification, seen map[string]bool, known bool) []Notification {
	if !known {
		return []Notification{}
	}
	return service.freshOf(items, seen)
}

func (service *NotificationRowService) freshOf(items []Notification, seen map[string]bool) []Notification {
	fresh := []Notification{}
	for _, item := range items {
		fresh = service.addWhenFresh(fresh, item, seen)
	}
	return fresh
}

func (service *NotificationRowService) addWhenFresh(fresh []Notification, item Notification, seen map[string]bool) []Notification {
	if item.ReadAt != "" || seen[item.NotificationId] {
		return fresh
	}
	return append(fresh, item)
}

func (service *NotificationRowService) withIds(seen map[string]bool, items []Notification) map[string]bool {
	for _, item := range items {
		seen[item.NotificationId] = true
	}
	return seen
}

func (service *NotificationRowService) readWhenClear(items []Notification, notificationId string, problem string) []Notification {
	if problem != "" {
		return items
	}
	return service.readOne(items, notificationId, service.times.Stamp())
}

func (service *NotificationRowService) readOne(items []Notification, notificationId string, stamp string) []Notification {
	read := []Notification{}
	for _, item := range items {
		read = append(read, service.readMatching(item, notificationId, stamp))
	}
	return read
}

func (service *NotificationRowService) readMatching(item Notification, notificationId string, stamp string) Notification {
	if item.NotificationId != notificationId {
		return item
	}
	item.ReadAt = stamp
	return item
}
