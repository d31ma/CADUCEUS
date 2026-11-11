package mailout

type QueueReport struct {
	Picked   int `json:"picked"`
	Sent     int `json:"sent"`
	Deferred int `json:"deferred"`
	Bounced  int `json:"bounced"`
	Expired  int `json:"expired"`
	Failed   int `json:"failed"`
}
