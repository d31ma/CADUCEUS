package search

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

type IndexStorage struct {
	Store DocumentStore
}

func (storage IndexStorage) Location(domainId string, messageId string) (Location, bool, error) {
	locations := []Location{}
	err := storage.Store.All(Collections{}.Locations(domainId), map[string]any{"domainId": domainId, "messageId": messageId}, &locations)
	return storage.merged(locations), len(locations) > 0, err
}

func (storage IndexStorage) Add(document Document, month string) error {
	err := storage.registerMonth(document.DomainId, month)
	if err != nil {
		return err
	}
	documentId, err := storage.Store.Insert(Collections{}.Documents(document.DomainId, month), document)
	if err != nil {
		return err
	}
	location := Location{DomainId: document.DomainId, MessageId: document.MessageId, Month: month, DocumentId: documentId}
	_, err = storage.Store.Insert(Collections{}.Locations(document.DomainId), location)
	return err
}

func (storage IndexStorage) Bury(domainId string, messageId string) error {
	_, err := storage.Store.Insert(Collections{}.Locations(domainId), Location{DomainId: domainId, MessageId: messageId, Deleted: true})
	return err
}

func (storage IndexStorage) Find(location Location) (Document, error) {
	document := Document{}
	err := storage.Store.Find(Collections{}.Documents(location.DomainId, location.Month), location.DocumentId, &document)
	if err != nil {
		return Document{}, err
	}
	return document, storage.sameDomain(document.DomainId, location.DomainId)
}

func (storage IndexStorage) Replace(location Location, document Document) error {
	return storage.Store.Replace(Collections{}.Documents(location.DomainId, location.Month), location.DocumentId, document)
}

func (storage IndexStorage) Live(location Location) bool {
	return !slices.Contains([]bool{location.DocumentId != "", !location.Deleted}, false)
}

func (storage IndexStorage) Months(domainId string) ([]string, error) {
	months := []Month{}
	err := storage.Store.All(Collections{}.Months(domainId), map[string]any{"domainId": domainId}, &months)
	return storage.newestFirst(months), err
}

func (storage IndexStorage) Documents(domainId string, month string, where map[string]any) ([]Document, error) {
	documents := []Document{}
	err := storage.Store.All(Collections{}.Documents(domainId, month), storage.inDomain(where, domainId), &documents)
	return documents, err
}

func (storage IndexStorage) registerMonth(domainId string, month string) error {
	known := []Month{}
	err := storage.Store.All(Collections{}.Months(domainId), map[string]any{"domainId": domainId, "month": month}, &known)
	if err != nil {
		return err
	}
	return storage.insertMonthUnless(len(known) > 0, domainId, month)
}

func (storage IndexStorage) insertMonthUnless(known bool, domainId string, month string) error {
	if known {
		return nil
	}
	_, err := storage.Store.Insert(Collections{}.Months(domainId), Month{DomainId: domainId, Month: month})
	return err
}

func (IndexStorage) sameDomain(stored string, wanted string) error {
	if stored != wanted {
		return Unavailable{Message: "the index entry belongs to another domain"}
	}
	return nil
}

func (storage IndexStorage) inDomain(where map[string]any, domainId string) map[string]any {
	scoped := maps.Clone(where)
	return storage.withDomain(scoped, domainId)
}

func (IndexStorage) withDomain(scoped map[string]any, domainId string) map[string]any {
	if scoped == nil {
		scoped = map[string]any{}
	}
	scoped["domainId"] = domainId
	return scoped
}

func (storage IndexStorage) merged(locations []Location) Location {
	merged := Location{}
	for _, location := range locations {
		merged = storage.combined(merged, location)
	}
	return merged
}

func (IndexStorage) combined(merged Location, location Location) Location {
	merged.DomainId = cmp.Or(merged.DomainId, location.DomainId)
	merged.MessageId = cmp.Or(merged.MessageId, location.MessageId)
	merged.Month = cmp.Or(merged.Month, location.Month)
	merged.DocumentId = cmp.Or(merged.DocumentId, location.DocumentId)
	merged.Deleted = merged.Deleted || location.Deleted
	return merged
}

func (IndexStorage) newestFirst(months []Month) []string {
	names := []string{}
	for _, month := range months {
		names = append(names, month.Month)
	}
	slices.SortFunc(names, func(left string, right string) int { return strings.Compare(right, left) })
	return slices.Compact(names)
}
