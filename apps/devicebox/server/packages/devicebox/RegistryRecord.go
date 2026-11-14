package devicebox

type RegistryRecord struct {
	Id       string `json:"id,omitempty"`
	Name     string `json:"name"`
	DomainId string `json:"domainId"`
}
