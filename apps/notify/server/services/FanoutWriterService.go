import (
	"cmp"
	"slices"
	"time"

	"mesail.dev/packages/notifications"
)

// @Service
type FanoutWriterService struct {
	// @Needs
	store *NotificationStoreService
	// @Needs
	states *NotificationStatesService
	// @Needs
	timestamps *TimestampService
}

func (writer *FanoutWriterService) Write(batch FanoutBatch, writeBudget int) (FanoutResult, error) {
	existing, failure := writer.store.FindByMessage(batch.DomainId, batch.Message.MessageId, writer.readLimit())
	fresh := writer.withoutExisting(batch.Recipients, writer.memberIds(existing))
	return writer.insertUnless(failure, batch, fresh, writeBudget)
}

func (writer *FanoutWriterService) memberIds(records []NotificationRecord) map[string]bool {
	memberIds := map[string]bool{}
	for _, record := range records {
		memberIds[record.MemberId] = true
	}
	return memberIds
}

func (writer *FanoutWriterService) withoutExisting(recipients []notifications.Recipient, existing map[string]bool) []notifications.Recipient {
	return slices.DeleteFunc(slices.Clone(recipients), func(recipient notifications.Recipient) bool {
		return existing[recipient.MemberId]
	})
}

func (writer *FanoutWriterService) insertUnless(failure error, batch FanoutBatch, fresh []notifications.Recipient, writeBudget int) (FanoutResult, error) {
	if failure != nil {
		return FanoutResult{}, failure
	}
	count := min(len(fresh), max(writeBudget, 0))
	inserted, failure := writer.insertAll(batch, fresh[:count])
	return FanoutResult{Inserted: inserted, Remaining: fresh[count:], Written: len(inserted)}, failure
}

func (writer *FanoutWriterService) insertAll(batch FanoutBatch, recipients []notifications.Recipient) ([]NotificationRecord, error) {
	createdAt := writer.createdAt(batch.Message.CreatedAt)
	inserted := []NotificationRecord{}
	failures := []error{}
	for _, recipient := range recipients {
		record, failure := writer.store.InsertIfAbsent(writer.record(batch, recipient, createdAt))
		inserted, failures = append(inserted, record), append(failures, failure)
	}
	return inserted, cmp.Or(failures...)
}

func (writer *FanoutWriterService) record(batch FanoutBatch, recipient notifications.Recipient, createdAt time.Time) NotificationRecord {
	message := batch.Message
	summaryDueAt := notifications.SummaryClock{}.DueAt(createdAt, batch.Settings[recipient.MemberId].EmailSummaryAfterMinutes)
	return NotificationRecord{
		DomainId:          batch.DomainId,
		MemberId:          recipient.MemberId,
		Kind:              recipient.Kind,
		ConversationId:    message.ConversationId,
		MessageId:         message.MessageId,
		ThreadRootId:      message.ThreadRootId,
		AuthorDisplayName: message.AuthorDisplayName,
		DedupeKey:         message.MessageId + "/" + recipient.MemberId,
		CreatedAt:         writer.timestamps.text(createdAt),
		PushState:         writer.states.PushPending(),
		SummaryDueAt:      summaryDueAt,
		SummaryState:      writer.summaryState(summaryDueAt),
	}
}

func (writer *FanoutWriterService) createdAt(text string) time.Time {
	moment, failure := writer.timestamps.Parse(text)
	if failure != nil {
		return time.Now()
	}
	return moment
}

func (writer *FanoutWriterService) summaryState(summaryDueAt int64) string {
	if summaryDueAt > 0 {
		return writer.states.SummaryWaiting()
	}
	return writer.states.SummaryNone()
}

func (writer *FanoutWriterService) readLimit() int {
	return 1000
}
