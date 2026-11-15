package backups

import (
	"mesail.dev/packages/attachments"
	"mesail.dev/packages/collections"
)

type Shelf struct {
	Documents Documents
	Scroll    Scroller
	Known     collections.KnownCollections
}

func (shelf Shelf) Find(domainId string, personId string) (Record, bool, error) {
	collection := shelf.collection(domainId)
	record := Record{}
	exists := false
	err := shelf.Known.Read(shelf.Documents, collection, func() error {
		var failure error
		exists, failure = Pages{Scroll: shelf.Scroll}.First(collection, map[string]any{"personId": personId}, &record)
		return failure
	})
	return record, exists, shelf.failure("find", err)
}

func (shelf Shelf) Save(record Record) (Record, error) {
	collection := shelf.collection(record.DomainId)
	id := record.ID
	record.ID = ""
	err := shelf.Known.Write(shelf.Documents, collection, func() error {
		var failure error
		id, failure = shelf.write(collection, id, record)
		return failure
	})
	record.ID = id
	return record, shelf.failure("save", err)
}

func (shelf Shelf) Delete(record Record) error {
	_, err := shelf.Documents.DeleteById(shelf.collection(record.DomainId), record.ID)
	return shelf.failure("delete", err)
}

func (shelf Shelf) write(collection string, id string, record Record) (string, error) {
	if id == "" {
		return shelf.Documents.Insert(collection, record)
	}
	_, err := shelf.Documents.Patch(collection, id, record, nil)
	return id, err
}

func (Shelf) failure(operation string, cause error) error {
	if cause != nil {
		return attachments.Unavailable{Operation: "backup " + operation + ": " + cause.Error()}
	}
	return nil
}

func (Shelf) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("backups", domainId)
}
