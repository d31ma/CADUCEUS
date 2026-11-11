package backend

type MessageRequest struct {
	ConnectionId string `json:"connectionId"`
	DomainId     string `json:"domainId"`
	Body         string `json:"body"`
}
