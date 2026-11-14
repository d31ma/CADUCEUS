import "mesail.dev/packages/events"

// @Service
type ChatPushesService struct {
	// @Needs
	members *MembershipService
	// @Needs
	pushes *ContentFreePushService
}

func (service *ChatPushesService) EnvelopeStored(envelope events.Envelope) error {
	payload := events.EnvelopeStored{}
	failure := events.PayloadReader{}.Read(envelope, &payload)
	people, failure := service.membersUnless(failure, envelope.DomainId, service.wanted(payload))
	_, failure = service.pushUnless(failure, ContentFreePush{DomainId: envelope.DomainId, PersonIds: people, Kind: service.pushes.NewMessage()})
	return failure
}

func (service *ChatPushesService) wanted(payload events.EnvelopeStored) []string {
	if payload.Kind != service.messageKind() {
		return []string{}
	}
	return []string{payload.ToPersonId}
}

func (service *ChatPushesService) membersUnless(failure error, domainId string, personIds []string) ([]string, error) {
	if failure != nil {
		return []string{}, failure
	}
	membership, problem := service.members.Split(domainId, personIds)
	return membership.Active, problem
}

func (service *ChatPushesService) pushUnless(failure error, push ContentFreePush) (CallPushReport, error) {
	if failure != nil || len(push.PersonIds) == 0 {
		return CallPushReport{}, failure
	}
	return service.pushes.Push(push)
}

func (service *ChatPushesService) messageKind() string {
	return "message"
}
