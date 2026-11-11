package realtime

import "mesail.dev/packages/collections"

type SeenBook struct {
	Storage Storage
}

func (book SeenBook) Seen(domainId string, eventId string) (bool, error) {
	collection := book.collection(domainId)
	err := book.shelf().Open(collection)
	if err != nil {
		return false, err
	}
	return book.shelf().First(collection, map[string]any{"eventId": eventId}, &SeenRecord{})
}

func (book SeenBook) Remember(domainId string, eventId string) error {
	seen, err := book.Seen(domainId, eventId)
	if err != nil {
		return err
	}
	return book.insert(domainId, eventId, seen)
}

func (book SeenBook) insert(domainId string, eventId string, seen bool) error {
	if seen {
		return nil
	}
	return book.shelf().Put(book.collection(domainId), "", SeenRecord{DomainId: domainId, EventId: eventId})
}

func (book SeenBook) shelf() DocumentShelf {
	return DocumentShelf{Storage: book.Storage}
}

func (SeenBook) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("events-seen", domainId)
}
