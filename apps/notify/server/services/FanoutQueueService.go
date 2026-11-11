import (
	"cmp"
	"encoding/json"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
)

// @Service
type FanoutQueueService struct {
	// @Needs
	store *FanoutStoreService
	// @Needs
	timestamps *TimestampService
}

func (queue *FanoutQueueService) Spill(batch FanoutBatch, remaining []notify.Recipient) error {
	message, failure := json.Marshal(batch.Message)
	recipients, encodeFailure := queue.encoded(remaining)
	return queue.insertUnless(cmp.Or(failure, encodeFailure), queue.record(batch, message, recipients))
}

func (queue *FanoutQueueService) Waiting(domainId string, limit int) ([]FanoutRecord, error) {
	return queue.store.Waiting(domainId, limit)
}

func (queue *FanoutQueueService) Read(record FanoutRecord) (events.MessageCreated, []notify.Recipient, error) {
	payload := events.MessageCreated{}
	failure := json.Unmarshal([]byte(record.Message), &payload)
	recipients, decodeFailure := queue.decoded(record.Recipients)
	return payload, recipients, cmp.Or(failure, decodeFailure)
}

func (queue *FanoutQueueService) Save(record FanoutRecord, remaining []notify.Recipient) error {
	recipients, failure := queue.encoded(remaining)
	return queue.saveUnless(failure, record, recipients)
}

func (queue *FanoutQueueService) record(batch FanoutBatch, message []byte, recipients []string) FanoutRecord {
	return FanoutRecord{
		DomainId:   batch.DomainId,
		MessageId:  batch.Message.MessageId,
		Message:    string(message),
		Recipients: recipients,
		CreatedAt:  queue.timestamps.Now(),
	}
}

func (queue *FanoutQueueService) insertUnless(failure error, record FanoutRecord) error {
	if failure != nil || len(record.Recipients) == 0 {
		return failure
	}
	_, failure = queue.store.InsertIfAbsent(record)
	return failure
}

func (queue *FanoutQueueService) saveUnless(failure error, record FanoutRecord, recipients []string) error {
	if failure != nil {
		return failure
	}
	return queue.store.SaveRecipients(record, recipients)
}

func (queue *FanoutQueueService) encoded(recipients []notify.Recipient) ([]string, error) {
	texts := []string{}
	failures := []error{}
	for _, recipient := range recipients {
		text, failure := json.Marshal(recipient)
		texts, failures = append(texts, string(text)), append(failures, failure)
	}
	return texts, cmp.Or(failures...)
}

func (queue *FanoutQueueService) decoded(texts []string) ([]notify.Recipient, error) {
	recipients := []notify.Recipient{}
	failures := []error{}
	for _, text := range texts {
		recipient := notify.Recipient{}
		failures = append(failures, json.Unmarshal([]byte(text), &recipient))
		recipients = append(recipients, recipient)
	}
	return recipients, cmp.Or(failures...)
}
