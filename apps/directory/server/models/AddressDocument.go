type AddressDocument struct {
	Id       string `json:"id,omitempty"`
	Address  string `json:"address"`
	DomainId string `json:"domainId"`
	Kind     string `json:"kind"`
	TargetId string `json:"targetId"`
}
