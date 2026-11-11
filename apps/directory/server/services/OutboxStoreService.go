import (
	"cmp"
	"errors"
	"slices"
	"time"

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
	waiting, failure := store.members.Waiting(domainId)
	due := store.due(waiting, time.Now())
	return due[:min(max(limit, 0), len(due))], failure
}

func (store *OutboxStoreService) due(documents []outbox.Document, now time.Time) []outbox.Document {
	return slices.DeleteFunc(documents, func(document outbox.Document) bool { return !(OutboxSchedule{}).Due(document, now) })
}

func (store *OutboxStoreService) Dead() ([]outbox.Document, error) {
	return store.withDead()
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

func (store *OutboxStoreService) withDead() ([]outbox.Document, error) {
	domains, failure := store.domains.All()
	documents, readFailure := store.documentsOf(domains)
	return documents, cmp.Or(failure, readFailure)
}

func (store *OutboxStoreService) documentsOf(domains []DomainDocument) ([]outbox.Document, error) {
	documents := []outbox.Document{}
	failures := []error{}
	for _, domain := range domains {
		found, failure := store.members.WithDead(domain.DomainId)
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
