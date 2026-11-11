package mailout

type RequeueRequest struct {
	DomainId   string `json:"domainId"`
	ItemId     string `json:"itemId"`
	EventId    string `json:"eventId"`
	Subscriber string `json:"subscriber"`
}
