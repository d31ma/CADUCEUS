package attachments

import (
	"regexp"
	"strings"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/collections"
	"mesail.dev/packages/ids"
)

type RecordBook struct {
	Documents Documents
	Now       func() time.Time
	Known     collections.KnownCollections
}

func (book RecordBook) Collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("attachments", domainId)
}

func (book RecordBook) Insert(record Record) (Record, error) {
	collection := book.Collection(record.DomainId)
	record.CreatedAt = book.Now().UTC().Format(time.RFC3339Nano)
	err := book.Known.Write(book.Documents, collection, func() error {
		id, failure := book.Documents.Insert(collection, record)
		record.ID = id
		return failure
	})
	return record, book.failure("insert", err)
}

func (book RecordBook) Lookup(domainId string, attachmentId string) (Record, error) {
	storedId, err := book.storedId(attachmentId)
	if err != nil {
		return Record{}, err
	}
	collection := book.Collection(domainId)
	record := Record{}
	exists := false
	err = book.Known.Read(book.Documents, collection, func() error {
		var failure error
		exists, failure = book.Documents.FindById(collection, storedId, &record)
		return failure
	})
	if err != nil {
		return Record{}, Unavailable{Operation: "find"}
	}
	record.ID = storedId
	return record, book.owned(record, exists, domainId)
}

func (book RecordBook) SetState(record Record, state string) (Record, error) {
	collection := book.Collection(record.DomainId)
	_, err := book.Documents.Patch(collection, record.ID, StatePatch{State: state}, nil)
	record.State = state
	return record, book.failure("state", err)
}

func (book RecordBook) View(record Record) View {
	return View{AttachmentId: book.WireId(record), FileName: record.FileName, MediaType: record.MediaType, Size: record.Size, Sha256: record.Sha256, State: record.State}
}

func (book RecordBook) WireId(record Record) string {
	return ids.WireId{}.Add(book.wirePrefix(), record.ID)
}

func (RecordBook) Key(record Record) blobs.Key {
	domainId, name, _ := strings.Cut(record.BlobKey, "/")
	return blobs.Key{DomainId: domainId, Name: name}
}

func (book RecordBook) storedId(attachmentId string) (string, error) {
	storedId, failure := ids.WireId{}.Strip(book.wirePrefix(), attachmentId)
	if failure != nil || !regexp.MustCompile(book.storedIdPattern()).MatchString(storedId) {
		return "", Missing{}
	}
	return storedId, nil
}

func (RecordBook) owned(record Record, exists bool, domainId string) error {
	if !exists || record.DomainId != domainId {
		return Missing{}
	}
	return nil
}

func (RecordBook) failure(operation string, cause error) error {
	if cause != nil {
		return Unavailable{Operation: operation}
	}
	return nil
}

func (RecordBook) wirePrefix() string {
	return "att"
}

func (RecordBook) storedIdPattern() string {
	return `^[A-Za-z0-9_-]{1,64}$`
}
