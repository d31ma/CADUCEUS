import (
	"cmp"
	"time"

	"mesail.dev/packages/notify"
)

// @Service
type NotificationStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
	// @Needs
	states *NotificationStatesService
	// @Needs
	timestamps *TimestampService
}

func (store *NotificationStoreService) InsertIfAbsent(record NotificationRecord) (NotificationRecord, error) {
	collection, failure := store.collectionOf(record)
	record.Id, failure = store.insertUnless(failure, collection, record)
	return record, failure
}

func (store *NotificationStoreService) Find(domainId string, storedId string) (NotificationRecord, bool, error) {
	records, failure := store.findInMonths(domainId, storedId)
	return store.first(records), len(records) > 0, failure
}

func (store *NotificationStoreService) ListForMember(domainId string, memberId string) ([]NotificationRecord, error) {
	return store.inMonths(domainId, map[string]any{"memberId": memberId}, store.readLimit())
}

func (store *NotificationStoreService) FindByMessage(domainId string, messageId string, limit int) ([]NotificationRecord, error) {
	records, failure := store.inMonths(domainId, map[string]any{"messageId": messageId}, limit)
	return store.trimmed(records, limit), failure
}

func (store *NotificationStoreService) FindUnreadInConversation(domainId string, memberId string, conversationId string, limit int) ([]NotificationRecord, error) {
	where := map[string]any{"memberId": memberId, "conversationId": conversationId}
	records, failure := store.inMonths(domainId, where, store.readLimit())
	return store.trimmed(store.unread(records), limit), failure
}

func (store *NotificationStoreService) FindPendingPushes(domainId string, limit int) ([]NotificationRecord, error) {
	records, failure := store.inMonths(domainId, map[string]any{"pushState": store.states.PushPending()}, limit)
	return store.trimmed(records, limit), failure
}

func (store *NotificationStoreService) FindDueSummaries(domainId string, now int64, limit int) ([]NotificationRecord, error) {
	where := map[string]any{"summaryState": store.states.SummaryWaiting()}
	records, failure := store.inMonths(domainId, where, store.readLimit())
	return store.trimmed(store.due(records, now), limit), failure
}

func (store *NotificationStoreService) MarkRead(record NotificationRecord, readAt string) error {
	collection, failure := store.collectionOf(record)
	return store.patchUnless(failure, collection, store.unreadId(record), store.readPatch(record, readAt))
}

func (store *NotificationStoreService) MarkAllRead(records []NotificationRecord, readAt string) error {
	return store.markAll(records, readAt)
}

func (store *NotificationStoreService) PatchPush(record NotificationRecord, state string, attempts int) error {
	collection, failure := store.collectionOf(record)
	return store.patchUnless(failure, collection, record.Id, NotificationPushPatch{PushState: state, PushAttempts: attempts})
}

func (store *NotificationStoreService) MarkSummarized(record NotificationRecord, summarizedAt string) error {
	collection, failure := store.collectionOf(record)
	patch := NotificationSummaryPatch{SummaryDueAt: 0, SummaryState: store.states.SummaryNone(), SummarizedAt: summarizedAt}
	return store.patchUnless(failure, collection, record.Id, patch)
}

func (store *NotificationStoreService) months() []string {
	return notify.Months{}.Recent(time.Now())
}

func (store *NotificationStoreService) markAll(records []NotificationRecord, readAt string) error {
	failures := []error{}
	for _, record := range records {
		failures = append(failures, store.MarkRead(record, readAt))
	}
	return cmp.Or(failures...)
}

func (store *NotificationStoreService) unreadId(record NotificationRecord) string {
	if record.ReadAt != "" {
		return ""
	}
	return record.Id
}

func (store *NotificationStoreService) collectionOf(record NotificationRecord) (string, error) {
	month, failure := store.timestamps.Month(record.CreatedAt)
	return store.names.Notifications(record.DomainId, month), failure
}

func (store *NotificationStoreService) insertUnless(failure error, collection string, record NotificationRecord) (string, error) {
	if failure != nil {
		return "", failure
	}
	return store.documents.InsertIfAbsent(collection, map[string]any{"dedupeKey": record.DedupeKey}, record)
}

func (store *NotificationStoreService) findInMonths(domainId string, storedId string) ([]NotificationRecord, error) {
	records := []NotificationRecord{}
	failures := []error{}
	for _, month := range store.months() {
		record := NotificationRecord{}
		found, failure := store.documents.Find(store.names.Notifications(domainId, month), storedId, &record)
		records, failures = append(records, store.ifFound(record, found)...), append(failures, failure)
	}
	return records, cmp.Or(failures...)
}

func (store *NotificationStoreService) ifFound(record NotificationRecord, found bool) []NotificationRecord {
	if found {
		return []NotificationRecord{record}
	}
	return []NotificationRecord{}
}

func (store *NotificationStoreService) inMonths(domainId string, where map[string]any, limit int) ([]NotificationRecord, error) {
	records := []NotificationRecord{}
	failures := []error{}
	for _, month := range store.months() {
		found := []NotificationRecord{}
		failure := store.documents.Matching(store.names.Notifications(domainId, month), where, limit, &found)
		records, failures = append(records, found...), append(failures, failure)
	}
	return records, cmp.Or(failures...)
}

func (store *NotificationStoreService) unread(records []NotificationRecord) []NotificationRecord {
	kept := []NotificationRecord{}
	for _, record := range records {
		kept = append(kept, store.ifUnread(record)...)
	}
	return kept
}

func (store *NotificationStoreService) ifUnread(record NotificationRecord) []NotificationRecord {
	if record.ReadAt == "" {
		return []NotificationRecord{record}
	}
	return []NotificationRecord{}
}

func (store *NotificationStoreService) due(records []NotificationRecord, now int64) []NotificationRecord {
	kept := []NotificationRecord{}
	for _, record := range records {
		kept = append(kept, store.ifDue(record, now)...)
	}
	return kept
}

func (store *NotificationStoreService) ifDue(record NotificationRecord, now int64) []NotificationRecord {
	if record.ReadAt == "" && record.SummaryDueAt > 0 && record.SummaryDueAt <= now {
		return []NotificationRecord{record}
	}
	return []NotificationRecord{}
}

func (store *NotificationStoreService) readPatch(record NotificationRecord, readAt string) NotificationReadPatch {
	return NotificationReadPatch{ReadAt: readAt, SummaryDueAt: 0, SummaryState: store.states.SummaryNone(), PushState: store.pushAfterRead(record.PushState)}
}

func (store *NotificationStoreService) pushAfterRead(state string) string {
	if state == store.states.PushPending() {
		return store.states.PushSkipped()
	}
	return state
}

func (store *NotificationStoreService) patchUnless(failure error, collection string, id string, patch any) error {
	if failure != nil || id == "" {
		return failure
	}
	return store.documents.Patch(collection, id, patch)
}

func (store *NotificationStoreService) first(records []NotificationRecord) NotificationRecord {
	return append(records, NotificationRecord{})[0]
}

func (store *NotificationStoreService) trimmed(records []NotificationRecord, limit int) []NotificationRecord {
	return records[:min(len(records), max(limit, 0))]
}

func (store *NotificationStoreService) readLimit() int {
	return 1000
}
