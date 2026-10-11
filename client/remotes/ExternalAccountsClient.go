// ExternalAccountsClient is the /api/v1/external-accounts route group. The
// sidebar lists the connected mailboxes and their provider folders, arranges
// them, and queues folder changes.
// @HTTPClient("/api/v1/external-accounts")
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
}
