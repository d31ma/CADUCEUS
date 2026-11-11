import "mesail.dev/packages/outbox"

// @Service
type MemberStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *MemberStorageService) Save(member MemberDocument) (outbox.Ref, error) {
	ref := storage.ref(member.DomainId, member.Id)
	member.Id = ""
	return ref, storage.documents.Replace(ref.Collection, ref.Id, member)
}

func (storage *MemberStorageService) Insert(member MemberDocument) (outbox.Ref, error) {
	id, failure := storage.documents.Insert(storage.names.Members(member.DomainId), member)
	return storage.ref(member.DomainId, id), failure
}

func (storage *MemberStorageService) Find(domainId string, memberId string) (MemberDocument, bool, error) {
	member := MemberDocument{}
	found, failure := storage.documents.FindOne(storage.names.Members(domainId), map[string]any{storage.memberField(): memberId}, &member)
	return member, found, failure
}

func (storage *MemberStorageService) All(domainId string) ([]MemberDocument, error) {
	members := []MemberDocument{}
	failure := storage.documents.All(storage.names.Members(domainId), map[string]any{}, &members)
	return members, failure
}

func (storage *MemberStorageService) ById(ref outbox.Ref) (MemberDocument, bool, error) {
	member := MemberDocument{}
	found, failure := storage.documents.ById(ref.Collection, ref.Id, &member)
	return member, found, failure
}

func (storage *MemberStorageService) Waiting(domainId string) ([]outbox.Document, error) {
	return storage.outboxWhere(domainId, storage.waitingField())
}

func (storage *MemberStorageService) WithDead(domainId string) ([]outbox.Document, error) {
	return storage.outboxWhere(domainId, storage.hasDeadField())
}

func (storage *MemberStorageService) SaveOutbox(ref outbox.Ref, fields outbox.Fields) error {
	document := OutboxFieldsDocument{PendingEvents: fields.PendingEvents, DeadEvents: fields.DeadEvents, OutboxPending: fields.OutboxPending, OutboxDead: fields.OutboxDead, OutboxWaiting: fields.OutboxPending > 0, OutboxHasDead: fields.OutboxDead > 0}
	return storage.documents.Replace(ref.Collection, ref.Id, document)
}

func (storage *MemberStorageService) outboxWhere(domainId string, flag string) ([]outbox.Document, error) {
	members := []MemberDocument{}
	failure := storage.documents.All(storage.names.Members(domainId), map[string]any{flag: true}, &members)
	return storage.asDocuments(storage.names.Members(domainId), members), failure
}

func (storage *MemberStorageService) asDocuments(collection string, members []MemberDocument) []outbox.Document {
	documents := []outbox.Document{}
	for _, member := range members {
		fields := outbox.Fields{PendingEvents: member.PendingEvents, DeadEvents: member.DeadEvents, OutboxPending: member.OutboxPending, OutboxDead: member.OutboxDead}
		documents = append(documents, outbox.Document{Ref: outbox.Ref{Collection: collection, Id: member.Id}, Fields: fields})
	}
	return documents
}

func (storage *MemberStorageService) ref(domainId string, id string) outbox.Ref {
	return outbox.Ref{Collection: storage.names.Members(domainId), Id: id}
}

func (storage *MemberStorageService) waitingField() string {
	return "outboxWaiting"
}

func (storage *MemberStorageService) hasDeadField() string {
	return "outboxHasDead"
}

func (storage *MemberStorageService) memberField() string {
	return "memberId"
}
