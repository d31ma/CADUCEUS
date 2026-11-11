type ThreadRecord struct {
	Id           string   `json:"id,omitempty"`
	DomainId     string   `json:"domainId"`
	ThreadRootId string   `json:"threadRootId"`
	MemberIds    []string `json:"memberIds"`
}
