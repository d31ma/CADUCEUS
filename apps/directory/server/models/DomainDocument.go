type DomainDocument struct {
	Id           string   `json:"id,omitempty"`
	DomainId     string   `json:"domainId"`
	Name         string   `json:"name"`
	Issuer       string   `json:"issuer"`
	State        string   `json:"state"`
	DkimSelector string   `json:"dkimSelector"`
	Dns          DnsState `json:"dns"`
}
