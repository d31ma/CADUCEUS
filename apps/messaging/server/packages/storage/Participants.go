package storage

import "mesail.dev/packages/core"

type Participants struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store Participants) OfConversation(domainId string, conversationId string) ([]core.Participant, error) {
	return store.matching(domainId, map[string]any{store.conversationField(): conversationId})
}

func (store Participants) OfMember(domainId string, memberId string) ([]core.Participant, error) {
	return store.matching(domainId, map[string]any{store.memberField(): memberId})
}

func (store Participants) Insert(participant core.Participant) (string, error) {
	return store.Tenant.InsertTracked(store.Names.Participants(participant.DomainId), participant, participant.OutboxPending)
}

func (store Participants) Save(participant core.Participant) error {
	return store.Tenant.ReplaceTracked(store.Names.Participants(participant.DomainId), participant.Id, participant, participant.OutboxPending)
}

func (store Participants) Delete(domainId string, id string) error {
	participant := core.Participant{}
	found, failure := store.Tenant.Find(store.Names.Participants(domainId), domainId, id, &participant)
	return store.deleteIf(found, failure, domainId, id)
}

func (store Participants) matching(domainId string, where map[string]any) ([]core.Participant, error) {
	participants := []core.Participant{}
	failure := store.Tenant.Matching(store.Names.Participants(domainId), domainId, where, &participants)
	return participants, failure
}

func (store Participants) deleteIf(found bool, failure error, domainId string, id string) error {
	if !found || failure != nil {
		return failure
	}
	return store.Tenant.Delete(store.Names.Participants(domainId), id)
}

func (Participants) conversationField() string {
	return "conversationId"
}

func (Participants) memberField() string {
	return "memberId"
}
