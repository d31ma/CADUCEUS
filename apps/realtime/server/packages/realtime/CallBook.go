package realtime

import (
	"slices"
	"time"
)

type CallBook struct {
	Storage Storage
}

func (book CallBook) Create(domainId string, record CallRecord) error {
	return book.shelf().Insert(domainId, record)
}

func (book CallBook) Find(domainId string, callId string) (CallRecord, error) {
	record := CallRecord{}
	found, err := book.shelf().Find(domainId, map[string]any{"callId": callId}, &record)
	if err != nil {
		return CallRecord{}, err
	}
	return record, book.present(found)
}

func (book CallBook) Touch(domainId string, record CallRecord, at string) error {
	return book.shelf().Patch(domainId, record.ID, CallChange{LastChangeAt: at})
}

func (book CallBook) Unchanged(domainId string, since time.Time) ([]CallRecord, error) {
	found := []CallRecord{}
	err := book.shelf().All(domainId, nil, &found)
	return slices.DeleteFunc(found, func(record CallRecord) bool { return TimeText{}.At(record.LastChangeAt).After(since) }), err
}

func (book CallBook) Delete(domainId string, record CallRecord) error {
	return book.shelf().Delete(domainId, record.ID)
}

func (CallBook) present(found bool) error {
	if !found {
		return MissingTarget{}.CallNotFound()
	}
	return nil
}

func (book CallBook) shelf() DomainShelf {
	return DomainShelf{Storage: book.Storage, Base: "calls"}
}
