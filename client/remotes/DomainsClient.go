// @HTTPClient({"setup":"SameOriginSetup"})
type DomainsClient interface {
	// @GetRequest("/domains")
	Domains(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]MailDomain, error)
}
