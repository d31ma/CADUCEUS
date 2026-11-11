type DeliveryAttempt struct {
	DomainId  string `json:"domainId"`
	ItemId    string `json:"itemId"`
	Rcpt      string `json:"rcpt"`
	Transport string `json:"transport"`
	Status    string `json:"status"`
	Code      int    `json:"code"`
	Enhanced  string `json:"enhanced"`
	Detail    string `json:"detail"`
	At        string `json:"at"`
}
