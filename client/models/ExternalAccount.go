type ExternalAccount struct {
	ID             string                      `json:"id"`
	MailboxAddress string                      `json:"mailboxAddress"`
	Provider       string                      `json:"provider"`
	State          string                      `json:"state"`
	Revision       float64                     `json:"revision"`
	Label          string                      `json:"label"`
	Color          string                      `json:"color"`
	Order          float64                     `json:"order"`
	Capabilities   ExternalAccountCapabilities `json:"capabilities"`
}
