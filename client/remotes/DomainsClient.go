// @HTTPClient
type DomainsClient interface {
	// @GetRequest("/domains")
	Domains(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]MailDomain, error)
}
