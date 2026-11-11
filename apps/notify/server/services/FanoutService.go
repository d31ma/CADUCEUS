import (
	"cmp"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
)

// @Service
type FanoutService struct {
	// @Needs
	queue *FanoutQueueService
	// @Needs
	recipients *RecipientService
	// @Needs
	writer *FanoutWriterService
}

func (service *FanoutService) Process(domainId string, writeBudget int) (JobProgress, error) {
	records, failure := service.queue.Waiting(domainId, service.waitingLimit())
	progress, failure := service.processUnless(failure, domainId, records, writeBudget)
	return service.withMore(progress, len(records)), failure
}

func (service *FanoutService) processUnless(failure error, domainId string, records []FanoutRecord, writeBudget int) (JobProgress, error) {
	if failure != nil {
		return JobProgress{}, failure
	}
	return service.processAll(domainId, records, writeBudget)
}

func (service *FanoutService) processAll(domainId string, records []FanoutRecord, writeBudget int) (JobProgress, error) {
	progress := JobProgress{}
	var failure error
	for _, record := range records {
		step, problem := service.processWithin(failure, domainId, record, writeBudget-progress.Written)
		progress, failure = service.added(progress, step), cmp.Or(failure, problem)
	}
	return progress, failure
}

func (service *FanoutService) processWithin(failure error, domainId string, record FanoutRecord, room int) (JobProgress, error) {
	if failure != nil || room < service.minimumRoom() {
		return JobProgress{}, failure
	}
	return service.processOne(domainId, record, room)
}

func (service *FanoutService) processOne(domainId string, record FanoutRecord, room int) (JobProgress, error) {
	payload, recipients, failure := service.queue.Read(record)
	batch, failure := service.resumeUnless(failure, domainId, payload, recipients)
	result, failure := service.writeUnless(failure, batch, room-service.saveWrites())
	failure = service.saveUnless(failure, record, result)
	return service.progressOf(result), failure
}

func (service *FanoutService) resumeUnless(failure error, domainId string, payload events.MessageCreated, recipients []notify.Recipient) (FanoutBatch, error) {
	if failure != nil {
		return FanoutBatch{}, failure
	}
	return service.recipients.Resume(domainId, payload, recipients)
}

func (service *FanoutService) writeUnless(failure error, batch FanoutBatch, writeBudget int) (FanoutResult, error) {
	if failure != nil {
		return FanoutResult{}, failure
	}
	return service.writer.Write(batch, writeBudget)
}

func (service *FanoutService) saveUnless(failure error, record FanoutRecord, result FanoutResult) error {
	if failure != nil {
		return failure
	}
	return service.queue.Save(record, result.Remaining)
}

func (service *FanoutService) progressOf(result FanoutResult) JobProgress {
	return JobProgress{Written: result.Written + service.saveWrites(), Done: service.finished(result)}
}

func (service *FanoutService) finished(result FanoutResult) int {
	if len(result.Remaining) == 0 {
		return 1
	}
	return 0
}

func (service *FanoutService) added(total JobProgress, step JobProgress) JobProgress {
	total.Written += step.Written
	total.Done += step.Done
	return total
}

func (service *FanoutService) withMore(progress JobProgress, waiting int) JobProgress {
	progress.More = progress.Done < waiting || waiting == service.waitingLimit()
	return progress
}

func (service *FanoutService) minimumRoom() int {
	return 2
}

func (service *FanoutService) saveWrites() int {
	return 1
}

func (service *FanoutService) waitingLimit() int {
	return 10
}
