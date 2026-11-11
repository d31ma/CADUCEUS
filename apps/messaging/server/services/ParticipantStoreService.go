import "mesail.dev/packages/core"

// @Service
type ParticipantStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *ParticipantStoreService) OfConversation(domainId string, conversationId string) ([]core.Participant, error) {
	return store.matching(domainId, map[string]any{store.conversationField(): conversationId})
}

func (store *ParticipantStoreService) OfMember(domainId string, memberId string) ([]core.Participant, error) {
	return store.matching(domainId, map[string]any{store.memberField(): memberId})
}

func (store *ParticipantStoreService) Insert(participant core.Participant) (string, error) {
	return store.tenant.InsertTracked(store.names.Participants(participant.DomainId), participant, participant.OutboxPending)
}

func (store *ParticipantStoreService) Save(participant core.Participant) error {
	return store.tenant.ReplaceTracked(store.names.Participants(participant.DomainId), participant.Id, participant, participant.OutboxPending)
}

func (store *ParticipantStoreService) Delete(domainId string, id string) error {
	participant := core.Participant{}
	found, failure := store.tenant.Find(store.names.Participants(domainId), domainId, id, &participant)
	return store.deleteIf(found, failure, domainId, id)
}

func (store *ParticipantStoreService) matching(domainId string, where map[string]any) ([]core.Participant, error) {
	participants := []core.Participant{}
	failure := store.tenant.Matching(store.names.Participants(domainId), domainId, where, &participants)
	return participants, failure
}

func (store *ParticipantStoreService) deleteIf(found bool, failure error, domainId string, id string) error {
	if !found || failure != nil {
		return failure
	}
	return store.tenant.Delete(store.names.Participants(domainId), id)
}

func (store *ParticipantStoreService) conversationField() string {
	return "conversationId"
}

func (store *ParticipantStoreService) memberField() string {
	return "memberId"
}
