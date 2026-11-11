package search

type Queries struct {
	Storage IndexStorage
	Reader  Reader
	Matches Matches
	Hits    Hits
}

func (queries Queries) Search(domainId string, authorization string, filter Filter) (Page, error) {
	readable, err := queries.Reader.Readable(authorization)
	if err != nil {
		return Page{}, Unavailable{Message: "conversation membership could not be checked", Cause: err}
	}
	months, err := queries.Storage.Months(domainId)
	if err != nil {
		return Page{}, err
	}
	found, err := queries.collect(domainId, queries.Matches.Months(months, filter), filter, readable)
	if err != nil {
		return Page{}, err
	}
	return queries.Hits.Page(found, filter), nil
}

func (queries Queries) collect(domainId string, months []string, filter Filter, readable map[string]bool) ([]Document, error) {
	found := []Document{}
	var err error
	for index := 0; index < len(months) && len(found) <= filter.Limit && err == nil; index++ {
		var matched []Document
		matched, err = queries.inMonth(domainId, months[index], filter, readable)
		found = append(found, matched...)
	}
	return found, err
}

func (queries Queries) inMonth(domainId string, month string, filter Filter, readable map[string]bool) ([]Document, error) {
	documents, err := queries.Storage.Documents(domainId, month, queries.Matches.Where(filter))
	return queries.Matches.Keep(documents, domainId, filter, readable), err
}
