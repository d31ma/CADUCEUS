package mailin

import (
	"regexp"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/ids"
)

type InboundBook struct {
	Storage Storage
}

func (InboundBook) Collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("inbound-raw", domainId)
}

func (book InboundBook) Find(domainId string, inboundId string) (InboundRaw, error) {
	storedId, err := book.storedId(inboundId)
	if err != nil {
		return InboundRaw{}, err
	}
	collection := book.Collection(domainId)
	exists, err := book.Storage.Documents.CollectionExists(collection)
	if err != nil {
		return InboundRaw{}, Unavailable{Operation: "collection check"}
	}
	return book.inCollection(collection, exists, storedId)
}

func (book InboundBook) inCollection(collection string, exists bool, storedId string) (InboundRaw, error) {
	if !exists {
		return InboundRaw{}, book.missing()
	}
	return book.document(collection, storedId)
}

func (book InboundBook) document(collection string, storedId string) (InboundRaw, error) {
	stored := InboundRaw{}
	found, err := book.Storage.Find(collection, storedId, &stored)
	stored.ID = storedId
	return stored, book.foundOrMissing(found, err)
}

func (book InboundBook) foundOrMissing(found bool, failure error) error {
	if failure == nil && !found {
		return book.missing()
	}
	return failure
}

func (book InboundBook) storedId(inboundId string) (string, error) {
	storedId, failure := ids.WireId{}.Strip(book.wirePrefix(), inboundId)
	if failure != nil || !regexp.MustCompile(book.storedIdPattern()).MatchString(storedId) {
		return "", book.missing()
	}
	return storedId, nil
}

func (InboundBook) missing() Missing {
	return Missing{Message: "no such inbound message"}
}

func (InboundBook) wirePrefix() string {
	return "in"
}

func (InboundBook) storedIdPattern() string {
	return `^[A-Za-z0-9_-]{1,64}$`
}
