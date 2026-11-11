import (
	"cmp"
	"errors"
	"slices"

	"mesail.dev/packages/outbox"
)

// @Service
type OutboxStoreService struct {
	// @Needs
	domains *DomainStorageService
	// @Needs
	members *MemberStorageService
	// @Needs
	names *CollectionNamesService
}

func (store *OutboxStoreService) PendingIn(domainId string, limit int) ([]outbox.Document, error) {
	documents, failure := store.members.Outbox(domainId)
	waiting := store.waiting(documents)
	return waiting[:min(max(limit, 0), len(waiting))], failure
}

func (store *OutboxStoreService) Dead() ([]outbox.Document, error) {
	documents, failure := store.all()
	return store.deadOnly(documents), failure
}

func (store *OutboxStoreService) Load(ref outbox.Ref) (outbox.Fields, error) {
	known, failure := store.known(ref)
	member, found, readFailure := store.readUnless(cmp.Or(failure, store.unknown(known)), ref)
	fields := outbox.Fields{PendingEvents: member.PendingEvents, DeadEvents: member.DeadEvents, OutboxPending: member.OutboxPending, OutboxDead: member.OutboxDead}
	return fields, cmp.Or(failure, store.unknown(known), readFailure, store.missing(found))
}

func (store *OutboxStoreService) Save(ref outbox.Ref, fields outbox.Fields) error {
	known, failure := store.known(ref)
	return store.saveUnless(cmp.Or(failure, store.unknown(known)), ref, fields)
}

func (store *OutboxStoreService) waiting(documents []outbox.Document) []outbox.Document {
	return slices.DeleteFunc(documents, func(document outbox.Document) bool { return document.Fields.OutboxPending == 0 })
}

func (store *OutboxStoreService) deadOnly(documents []outbox.Document) []outbox.Document {
	return slices.DeleteFunc(documents, func(document outbox.Document) bool { return document.Fields.OutboxDead == 0 })
}

func (store *OutboxStoreService) all() ([]outbox.Document, error) {
	domains, failure := store.domains.All()
	documents, readFailure := store.documentsOf(domains)
	return documents, cmp.Or(failure, readFailure)
}

func (store *OutboxStoreService) documentsOf(domains []DomainDocument) ([]outbox.Document, error) {
	documents := []outbox.Document{}
	failures := []error{}
	for _, domain := range domains {
		found, failure := store.members.Outbox(domain.DomainId)
		documents, failures = append(documents, found...), append(failures, failure)
	}
	return documents, errors.Join(failures...)
}

func (store *OutboxStoreService) known(ref outbox.Ref) (bool, error) {
	domains, failure := store.domains.All()
	return slices.ContainsFunc(domains, func(domain DomainDocument) bool { return store.names.Members(domain.DomainId) == ref.Collection }), failure
}

func (store *OutboxStoreService) readUnless(refusal error, ref outbox.Ref) (MemberDocument, bool, error) {
	if refusal != nil {
		return MemberDocument{}, false, nil
	}
	return store.members.ById(ref)
}

func (store *OutboxStoreService) saveUnless(refusal error, ref outbox.Ref, fields outbox.Fields) error {
	if refusal != nil {
		return refusal
	}
	return store.members.SaveOutbox(ref, fields)
}

func (store *OutboxStoreService) unknown(known bool) error {
	if known {
		return nil
	}
	return &NotFound{Message: "no such outbox collection"}
}

func (store *OutboxStoreService) missing(found bool) error {
	if found {
		return nil
	}
	return &NotFound{Message: "no such outbox document"}
}
