// @HTTPClient("/api/v1")
type ExternalAccountsClient interface {
	// @GetRequest("/external-accounts")
	Accounts(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/provider-discovery")
	Discover(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (DiscoveredSettings, error)

	// @PostRequest("/external-accounts/oauth/google/begin")
	BeginGoogle(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PatchRequest("/external-accounts/{id}")
	Change(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts/oauth/microsoft/begin")
	BeginMicrosoft(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts/oauth/google/complete")
	CompleteGoogle(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/provider-catalog")
	Providers(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts/oauth/microsoft/complete")
	CompleteMicrosoft(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts/{id}/retry")
	Retry(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts/validate")
	Validate(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts")
	Create(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/external-accounts/{id}/reconnect")
	Reconnect(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/external-accounts/{id}/diagnostics")
	Diagnostics(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/external-accounts/{id}")
	Remove(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)
}
