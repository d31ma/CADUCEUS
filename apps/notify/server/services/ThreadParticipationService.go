import "mesail.dev/packages/events"

// @Service
type ThreadParticipationService struct {
	// @Needs
	threads *ThreadStoreService
}

func (service *ThreadParticipationService) Record(domainId string, payload events.MessageCreated) ([]string, error) {
	threadRootId := service.rootOf(payload)
	participants, failure := service.threads.Participants(domainId, threadRootId)
	return participants, service.addUnless(failure, domainId, threadRootId, payload)
}

func (service *ThreadParticipationService) rootOf(payload events.MessageCreated) string {
	if payload.ThreadRootId == "" {
		return payload.MessageId
	}
	return payload.ThreadRootId
}

func (service *ThreadParticipationService) addUnless(failure error, domainId string, threadRootId string, payload events.MessageCreated) error {
	if failure != nil || payload.AuthorKind != service.memberAuthor() {
		return failure
	}
	return service.threads.AddParticipant(domainId, threadRootId, payload.AuthorMemberId)
}

func (service *ThreadParticipationService) memberAuthor() string {
	return "member"
}
