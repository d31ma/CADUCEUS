package mailout

type Suppression struct {
	DomainId    string `json:"domainId"`
	Address     string `json:"address"`
	AddressHash string `json:"addressHash"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"createdAt"`
}
