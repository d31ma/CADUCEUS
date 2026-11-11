package backend

type DisconnectRequest struct {
	ConnectionId string `json:"connectionId"`
	DomainId     string `json:"domainId"`
}
