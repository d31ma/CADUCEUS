package realtime

import (
	"time"

	"mesail.dev/packages/collections"
)

type PresenceBook struct {
	Storage Storage
}

func (book PresenceBook) Find(domainId string, memberId string) (PresenceRecord, bool, error) {
	collection, err := book.open(domainId)
	if err != nil {
		return PresenceRecord{}, false, err
	}
	return book.first(collection, memberId)
}

func (book PresenceBook) Save(record PresenceRecord) error {
	existing, _, err := book.Find(record.DomainId, record.MemberId)
	if err != nil {
		return err
	}
	record.ID = ""
	return book.shelf().Put(book.collection(record.DomainId), existing.ID, record)
}

func (book PresenceBook) ForMembers(domainId string, memberIds []string) ([]PresenceRecord, error) {
	collection, err := book.open(domainId)
	if err != nil {
		return nil, err
	}
	return book.collect(collection, memberIds)
}

func (book PresenceBook) Due(domainId string, now string, limit int) ([]PresenceRecord, error) {
	at, err := book.time(now)
	if err != nil {
		return nil, err
	}
	collection, err := book.open(domainId)
	if err != nil {
		return nil, err
	}
	return book.dueInStates(collection, PresenceDue{Now: at}, limit)
}

func (book PresenceBook) collect(collection string, memberIds []string) ([]PresenceRecord, error) {
	records := []PresenceRecord{}
	for _, memberId := range memberIds {
		record, found, err := book.first(collection, memberId)
		if err != nil {
			return nil, err
		}
		records = book.keep(records, record, found)
	}
	return records, nil
}

func (PresenceBook) keep(records []PresenceRecord, record PresenceRecord, found bool) []PresenceRecord {
	if found {
		return append(records, record)
	}
	return records
}

func (book PresenceBook) dueInStates(collection string, rule PresenceDue, limit int) ([]PresenceRecord, error) {
	due := []PresenceRecord{}
	for _, state := range (PresenceStates{}).Online() {
		found, err := book.dueInState(collection, state, rule, limit-len(due))
		if err != nil {
			return nil, err
		}
		due = append(due, found...)
	}
	return due, nil
}

func (book PresenceBook) dueInState(collection string, state string, rule PresenceDue, limit int) ([]PresenceRecord, error) {
	due := []PresenceRecord{}
	cursor := ""
	for page := 0; book.morePages(page, cursor, len(due), limit); page++ {
		records := []PresenceRecord{}
		next, err := book.Storage.Page(Query{Collection: collection, Where: map[string]any{"state": state}, Cursor: cursor, Size: book.pageSize()}, &records)
		if err != nil {
			return nil, err
		}
		due = append(due, book.dueOnly(records, rule)...)
		cursor = next
	}
	return due[:min(len(due), max(limit, 0))], nil
}

func (book PresenceBook) morePages(page int, cursor string, count int, limit int) bool {
	return count < limit && (page == 0 || cursor != "") && page < book.maximumPages()
}

func (book PresenceBook) dueOnly(records []PresenceRecord, rule PresenceDue) []PresenceRecord {
	due := []PresenceRecord{}
	for _, record := range records {
		due = book.keep(due, record, rule.Due(record))
	}
	return due
}

func (book PresenceBook) first(collection string, memberId string) (PresenceRecord, bool, error) {
	record := PresenceRecord{}
	found, err := book.shelf().First(collection, map[string]any{"memberId": memberId}, &record)
	return record, found, err
}

func (book PresenceBook) open(domainId string) (string, error) {
	collection := book.collection(domainId)
	return collection, book.shelf().Open(collection)
}

func (PresenceBook) time(now string) (time.Time, error) {
	at, err := time.Parse(time.RFC3339, now)
	if err != nil {
		return at, Invalid{Field: "now", Reason: "must be an RFC 3339 time"}
	}
	return at, nil
}

func (book PresenceBook) shelf() DocumentShelf {
	return DocumentShelf{Storage: book.Storage}
}

func (PresenceBook) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("presence", domainId)
}

func (PresenceBook) pageSize() int {
	return 100
}

func (PresenceBook) maximumPages() int {
	return 10
}
