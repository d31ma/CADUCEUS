type MailDomain struct {
	Domain string        `json:"domain"`
	Routes []DomainRoute `json:"routes"`
}
