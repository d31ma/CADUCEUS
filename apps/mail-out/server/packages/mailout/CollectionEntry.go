package mailout

type CollectionEntry struct {
	Collection string `json:"collection"`
	Base       string `json:"base"`
	DomainId   string `json:"domainId"`
}
