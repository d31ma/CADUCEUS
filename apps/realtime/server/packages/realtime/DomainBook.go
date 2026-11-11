package realtime

type DomainBook struct {
	Storage Storage
}

func (book DomainBook) Remember(domainId string) error {
	err := book.keeper().Ensure(book.collection())
	if err != nil {
		return err
	}
	found := []DomainRecord{}
	_, err = book.Storage.Page(Query{Collection: book.collection(), Where: map[string]any{"domainId": domainId}, Size: 1}, &found)
	if err != nil {
		return Unavailable{Operation: "domain find"}
	}
	return book.insertUnless(found, domainId)
}

func (book DomainBook) List(cursor string, limit int) ([]string, string, error) {
	exists, err := book.keeper().Exists(book.collection())
	if err != nil || !exists {
		return []string{}, "", err
	}
	return book.page(cursor, limit)
}

func (book DomainBook) page(cursor string, limit int) ([]string, string, error) {
	found := []DomainRecord{}
	next, err := book.Storage.Page(Query{Collection: book.collection(), Cursor: cursor, Size: min(max(limit, 1), book.maximumPageSize())}, &found)
	if err != nil {
		return nil, "", Unavailable{Operation: "domain scroll"}
	}
	return book.domainIds(found), next, nil
}

func (DomainBook) domainIds(found []DomainRecord) []string {
	domainIds := []string{}
	for _, record := range found {
		domainIds = append(domainIds, record.DomainId)
	}
	return domainIds
}

func (book DomainBook) insertUnless(found []DomainRecord, domainId string) error {
	if len(found) > 0 {
		return nil
	}
	_, err := book.Storage.Documents.Insert(book.collection(), DomainRecord{DomainId: domainId})
	if err != nil {
		return Unavailable{Operation: "domain insert"}
	}
	return nil
}

func (book DomainBook) keeper() CollectionKeeper {
	return CollectionKeeper{Documents: book.Storage.Documents}
}

func (DomainBook) collection() string {
	return "realtime-domains"
}

func (DomainBook) maximumPageSize() int {
	return 100
}
