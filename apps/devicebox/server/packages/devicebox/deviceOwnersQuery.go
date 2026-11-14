package devicebox

type deviceOwnersQuery struct {
	DomainId  string   `json:"domainId"`
	DeviceIds []string `json:"deviceIds"`
}
