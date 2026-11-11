package reception

type Address struct {
	Address  string `json:"address"`
	DomainId string `json:"domainId"`
	Kind     string `json:"kind"`
	TargetId string `json:"targetId"`
}
