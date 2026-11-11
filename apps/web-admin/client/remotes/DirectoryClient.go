// @HTTPClient("/api/v1/directory")
type DirectoryClient interface {
	// @GetRequest("/domains")
	Domains(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]Domain, error)

	// @PostRequest("/domains")
	CreateDomain(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		request BehaviorValue,
	) (Domain, error)

	// @PostRequest("/domains/{domainId}/verify")
	VerifyDomain(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		domainId string,
	) (Domain, error)

	// @DeleteRequest("/domains/{domainId}")
	RemoveDomain(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		domainId BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/members")
	Members(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("query")
		query string,
	) (MemberPage, error)

	// @PostRequest("/aliases")
	CreateAlias(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		request BehaviorValue,
	) (Address, error)
}
