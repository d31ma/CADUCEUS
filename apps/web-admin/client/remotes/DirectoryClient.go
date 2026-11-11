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

	// @PostRequest("/members")
	AddMember(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		request BehaviorValue,
	) (AddedMember, error)

	// @PostRequest("/domains/{domainId}/members")
	AddDomainMember(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		domainId string,
		// @RequestBody
		request BehaviorValue,
	) (AddedMember, error)

	// @PatchRequest("/members/{memberId}")
	ChangeMember(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		memberId string,
		// @RequestBody
		request BehaviorValue,
	) (MemberSummary, error)

	// @DeleteRequest("/members/{memberId}")
	RemoveMember(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		memberId BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/people")
	People(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("query")
		query string,
	) (PersonPage, error)

	// @PatchRequest("/people/{personId}")
	ChangePerson(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		personId string,
		// @RequestBody
		request BehaviorValue,
	) (PersonSummary, error)

	// @PostRequest("/people/{personId}/password")
	ResetPassword(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		personId BehaviorValue,
		// @RequestBody
		request BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/people/{personId}/totp")
	RemoveTwoStepCode(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		personId BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/people/{personId}")
	DeletePerson(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		personId BehaviorValue,
	) (BehaviorValue, error)
}
