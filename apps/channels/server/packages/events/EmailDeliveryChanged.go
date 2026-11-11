package events

type EmailDeliveryChanged struct {
	MessageId string `json:"messageId"`
	Rcpt      string `json:"rcpt"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}
