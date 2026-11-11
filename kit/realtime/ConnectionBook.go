package realtime

import (
	"slices"
	"time"

	"mesail.dev/packages/collections"
)

type ConnectionBook struct {
	Storage Storage
	Now     func() time.Time
}

func (book ConnectionBook) Add(connection Connection) error {
	collection := book.collection(connection.DomainId)
	err := book.keeper().Ensure(collection)
	if err != nil {
		return err
	}
	found, err := book.read(collection, book.byConnection(connection.ConnectionId))
	if err != nil {
		return err
	}
	return book.insertUnless(collection, found, connection)
}

func (book ConnectionBook) Find(domainId string, connectionId string) (Connection, error) {
	found, err := book.scan(domainId, book.byConnection(connectionId))
	if err != nil {
		return Connection{}, err
	}
	return book.first(found)
}

func (book ConnectionBook) Renew(domainId string, connectionId string, expiresAt string) error {
	connection, err := book.Find(domainId, connectionId)
	if err != nil {
		return err
	}
	_, err = book.Storage.Documents.Patch(book.collection(domainId), connection.ID, ConnectionRenewal{ExpiresAt: expiresAt}, nil)
	return book.failure("connection renew", err)
}

func (book ConnectionBook) Remove(domainId string, connectionId string) (Connection, error) {
	found, err := book.scan(domainId, book.byConnection(connectionId))
	if err != nil {
		return Connection{}, err
	}
	return book.deleteAll(book.collection(domainId), found)
}

func (book ConnectionBook) CountForMember(domainId string, memberId string) (int, error) {
	found, err := book.scan(domainId, map[string]any{book.memberField(): memberId})
	return len(found), err
}

func (book ConnectionBook) ForMembers(domainId string, memberIds []string) ([]Connection, error) {
	found, err := book.InDomain(domainId)
	if err != nil {
		return nil, err
	}
	return book.ofMembers(found, memberIds), nil
}

func (book ConnectionBook) InDomain(domainId string) ([]Connection, error) {
	return book.scan(domainId, map[string]any{book.domainField(): domainId})
}

func (book ConnectionBook) Expired(domainId string, now string, limit int) ([]Connection, error) {
	found, err := book.InDomain(domainId)
	if err != nil {
		return nil, err
	}
	expired := book.expiredBy(found, book.moment(now))
	return expired[:min(len(expired), max(limit, 0))], nil
}

func (book ConnectionBook) scan(domainId string, where map[string]any) ([]Connection, error) {
	collection := book.collection(domainId)
	exists, err := book.keeper().Exists(collection)
	if err != nil || !exists {
		return []Connection{}, err
	}
	return book.read(collection, where)
}

func (book ConnectionBook) read(collection string, where map[string]any) ([]Connection, error) {
	found, cursor, more := []Connection{}, "", true
	for pages := 0; more && pages < book.maximumPages(); pages++ {
		page := []Connection{}
		next, err := book.Storage.Page(Query{Collection: collection, Where: where, Cursor: cursor, Size: book.pageSize()}, &page)
		if err != nil {
			return nil, Unavailable{Operation: "connection scroll"}
		}
		found, cursor, more = append(found, page...), next, next != ""
	}
	return found, nil
}

func (book ConnectionBook) insertUnless(collection string, found []Connection, connection Connection) error {
	if len(found) > 0 {
		return nil
	}
	connection.ID = ""
	connection.ConnectedAt = book.Now().UTC().Format(time.RFC3339)
	_, err := book.Storage.Documents.Insert(collection, connection)
	return book.failure("connection insert", err)
}

func (book ConnectionBook) deleteAll(collection string, found []Connection) (Connection, error) {
	removed := Connection{}
	for _, connection := range found {
		_, err := book.Storage.Documents.DeleteById(collection, connection.ID)
		if err != nil {
			return Connection{}, Unavailable{Operation: "connection delete"}
		}
		removed = connection
	}
	return removed, nil
}

func (ConnectionBook) first(found []Connection) (Connection, error) {
	if len(found) == 0 {
		return Connection{}, Missing{What: "connection"}
	}
	return found[0], nil
}

func (ConnectionBook) ofMembers(found []Connection, memberIds []string) []Connection {
	return slices.DeleteFunc(found, func(connection Connection) bool {
		return !slices.Contains(memberIds, connection.MemberId)
	})
}

func (book ConnectionBook) expiredBy(found []Connection, now time.Time) []Connection {
	return slices.DeleteFunc(found, func(connection Connection) bool {
		return book.moment(connection.ExpiresAt).After(now)
	})
}

func (ConnectionBook) moment(text string) time.Time {
	moment, _ := time.Parse(time.RFC3339, text)
	return moment
}

func (ConnectionBook) failure(operation string, err error) error {
	if err != nil {
		return Unavailable{Operation: operation}
	}
	return nil
}

func (book ConnectionBook) keeper() CollectionKeeper {
	return CollectionKeeper{Documents: book.Storage.Documents}
}

func (book ConnectionBook) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain(book.base(), domainId)
}

func (book ConnectionBook) byConnection(connectionId string) map[string]any {
	return map[string]any{book.connectionField(): connectionId}
}

func (ConnectionBook) base() string {
	return "connections"
}

func (ConnectionBook) connectionField() string {
	return "connectionId"
}

func (ConnectionBook) memberField() string {
	return "memberId"
}

func (ConnectionBook) domainField() string {
	return "domainId"
}

func (ConnectionBook) pageSize() int {
	return 100
}

func (ConnectionBook) maximumPages() int {
	return 100
}
