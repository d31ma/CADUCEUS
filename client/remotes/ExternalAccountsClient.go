// ExternalAccountsClient is the /api/v1/external-accounts route group. The
// sidebar lists the connected mailboxes and their provider folders, arranges
// them, and queues folder changes. Settings connects, reconnects, pauses,
// diagnoses and removes them.
// @HTTPClient({"path":"/api/v1/external-accounts","setup":"SameOriginSetup"})
type ExternalAccountsClient interface {
	// @GetRequest
	Accounts(
		// @RequestHeader("Authorization")
		authorization string,
	) (ExternalAccountList, error)

	// @GetRequest("/{id}/folders")
	Folders(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (ExternalFolderList, error)

	// @PatchRequest("/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		update BehaviorValue,
	) (ExternalAccount, error)

	// @PostRequest("/{id}/retry")
	Retry(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		retry BehaviorValue,
	) (ExternalRetryAnswer, error)

	// @PostRequest("/{id}/mutations")
	Mutate(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		mutation BehaviorValue,
	) (MutationAnswer, error)

	// @PostRequest
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		account BehaviorValue,
	) (ExternalAccount, error)

	// @PostRequest("/validate")
	Validate(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		account BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/{id}/reconnect")
	Reconnect(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		credentials BehaviorValue,
	) (ExternalAccount, error)

	// @DeleteRequest("/{id}")
	Remove(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable
		id BehaviorValue,
		// @RequestBody
		revision BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/{id}/diagnostics")
	Diagnostics(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (AccountSyncReport, error)

	// @PostRequest("/oauth/google/begin")
	BeginGoogle(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		request BehaviorValue,
	) (ProviderAuthorization, error)

	// @PostRequest("/oauth/microsoft/begin")
	BeginMicrosoft(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		request BehaviorValue,
	) (ProviderAuthorization, error)

	// @PostRequest("/oauth/google/complete")
	CompleteGoogle(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		callback BehaviorValue,
	) (ExternalAccount, error)

	// @PostRequest("/oauth/microsoft/complete")
	CompleteMicrosoft(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		callback BehaviorValue,
	) (ExternalAccount, error)
}
