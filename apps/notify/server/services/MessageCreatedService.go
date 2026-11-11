import "mesail.dev/packages/events"

// @Service
type MessageCreatedService struct {
	// @Needs
	threads *ThreadParticipationService
	// @Needs
	recipients *RecipientService
	// @Needs
	writer *FanoutWriterService
	// @Needs
	queue *FanoutQueueService
	// @Needs
	pushes *PushService
}

func (service *MessageCreatedService) Apply(domainId string, payload events.MessageCreated) error {
	participants, failure := service.threads.Record(domainId, payload)
	batch, failure := service.batchUnless(failure, domainId, payload, participants)
	result, failure := service.writeUnless(failure, batch)
	failure = service.spillUnless(failure, batch, result)
	return service.pushUnless(failure, domainId, result)
}

func (service *MessageCreatedService) batchUnless(failure error, domainId string, payload events.MessageCreated, participants []string) (FanoutBatch, error) {
	if failure != nil {
		return FanoutBatch{}, failure
	}
	return service.recipients.For(domainId, payload, participants)
}

func (service *MessageCreatedService) writeUnless(failure error, batch FanoutBatch) (FanoutResult, error) {
	if failure != nil {
		return FanoutResult{}, failure
	}
	return service.writer.Write(batch, service.notificationWriteLimit())
}

func (service *MessageCreatedService) spillUnless(failure error, batch FanoutBatch, result FanoutResult) error {
	if failure != nil {
		return failure
	}
	return service.queue.Spill(batch, result.Remaining)
}

func (service *MessageCreatedService) pushUnless(failure error, domainId string, result FanoutResult) error {
	if failure != nil {
		return failure
	}
	_, failure = service.pushes.Deliver(domainId, result.Inserted, service.pushWriteBudget(result))
	return failure
}

func (service *MessageCreatedService) pushWriteBudget(result FanoutResult) int {
	return service.callWriteLimit() - service.reservedWrites() - result.Written
}

func (service *MessageCreatedService) callWriteLimit() int {
	return 25
}

func (service *MessageCreatedService) notificationWriteLimit() int {
	return 20
}

func (service *MessageCreatedService) reservedWrites() int {
	return 2
}
