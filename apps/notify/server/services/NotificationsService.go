import (
	"cmp"
	"errors"

	"mesail.dev/packages/ids"
	"mesail.dev/packages/notifications"
	"mesail.dev/packages/oidc"
)

// @Service
type NotificationsService struct {
	// @Needs
	callers *CallerService
	// @Needs
	store *NotificationStoreService
	// @Needs
	timestamps *TimestampService
}

func (service *NotificationsService) List(authorization string, cursor string, limitText string) (NotificationPage, error) {
	caller, failure := service.callers.Caller(authorization)
	limit, invalidLimit := notifications.PageRequest{}.Limit(limitText)
	failure = cmp.Or(failure, service.invalidPage(invalidLimit, "limit"))
	records, failure := service.recordsUnless(failure, caller)
	page, failure := service.pageUnless(failure, records, cursor, limit)
	return page, service.invalidPage(failure, "cursor")
}

func (service *NotificationsService) MarkRead(authorization string, notificationId string) error {
	caller, failure := service.callers.Caller(authorization)
	storedId, invalidId := ids.WireId{}.Strip(service.prefix(), notificationId)
	failure = cmp.Or(failure, service.missing(invalidId))
	record, found, failure := service.findUnless(failure, caller.DomainId, storedId)
	failure = cmp.Or(failure, service.ownership(caller, record, found))
	return service.markUnless(failure, record)
}

func (service *NotificationsService) recordsUnless(failure error, caller oidc.Caller) ([]NotificationRecord, error) {
	if failure != nil {
		return nil, failure
	}
	return service.store.ListForMember(caller.DomainId, caller.MemberId)
}

func (service *NotificationsService) pageUnless(failure error, records []NotificationRecord, cursor string, limit int) (NotificationPage, error) {
	if failure != nil {
		return NotificationPage{Items: []Notification{}}, failure
	}
	indexes, next, problem := notifications.Newest{}.Page(service.sortKeys(records), cursor, limit)
	return NotificationPage{Items: service.wireItems(records, indexes), NextCursor: next}, problem
}

func (service *NotificationsService) sortKeys(records []NotificationRecord) []notifications.Sortable {
	keys := []notifications.Sortable{}
	for _, record := range records {
		keys = append(keys, notifications.Sortable{CreatedAt: record.CreatedAt, Id: record.Id})
	}
	return keys
}

func (service *NotificationsService) wireItems(records []NotificationRecord, indexes []int) []Notification {
	items := []Notification{}
	for _, index := range indexes {
		items = append(items, service.wire(records[index]))
	}
	return items
}

func (service *NotificationsService) wire(record NotificationRecord) Notification {
	return Notification{
		NotificationId: ids.WireId{}.Add(service.prefix(), record.Id),
		Kind:           record.Kind,
		ConversationId: record.ConversationId,
		MessageId:      record.MessageId,
		CreatedAt:      record.CreatedAt,
		ReadAt:         record.ReadAt,
	}
}

func (service *NotificationsService) findUnless(failure error, domainId string, storedId string) (NotificationRecord, bool, error) {
	if failure != nil {
		return NotificationRecord{}, false, failure
	}
	return service.store.Find(domainId, storedId)
}

func (service *NotificationsService) ownership(caller oidc.Caller, record NotificationRecord, found bool) error {
	if !found || record.MemberId != caller.MemberId || record.DomainId != caller.DomainId {
		return service.missingNotification()
	}
	return nil
}

func (service *NotificationsService) markUnless(failure error, record NotificationRecord) error {
	if failure != nil {
		return failure
	}
	return service.store.MarkRead(record, service.timestamps.Now())
}

func (service *NotificationsService) missing(failure error) error {
	if failure != nil {
		return service.missingNotification()
	}
	return nil
}

func (service *NotificationsService) invalidPage(failure error, field string) error {
	if errors.As(failure, &notifications.InvalidPage{}) {
		return &InvalidRequest{Field: field, Reason: failure.Error()}
	}
	return failure
}

func (service *NotificationsService) missingNotification() error {
	return &NotFound{What: "notification"}
}

func (service *NotificationsService) prefix() string {
	return "ntf"
}
