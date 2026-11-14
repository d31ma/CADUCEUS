package realtime

type DeviceOwnersQuery struct {
	DomainId  string   `json:"domainId"`
	DeviceIds []string `json:"deviceIds"`
}
