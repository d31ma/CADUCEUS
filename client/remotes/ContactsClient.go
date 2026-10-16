// ContactsClient is the /contacts route group: the contact list, its groups,
// the CardDAV address books and their queued changes, vCard import and
// export, and the composer's address suggestions.
// @HTTPClient("/contacts")
type ContactsClient interface {
	// @GetRequest
	List(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
		// @RequestParam("source")
		source string,
		// @RequestParam("groupId")
		groupID string,
	) ([]Contact, error)

	// @PostRequest
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		draft BehaviorValue,
	) (Contact, error)

	// @PutRequest("/{id}")
	Update(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		draft BehaviorValue,
	) (Contact, error)

	// @DeleteRequest("/{id}")
	Delete(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestParam("idempotencyKey")
		idempotencyKey string,
	) (ContactDeletion, error)

	// @GetRequest("/{id}/mutations/{mutationId}")
	Mutation(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @PathVariable("mutationId")
		mutationID string,
		// @RequestParam("accountId")
		accountID string,
	) (ContactMutation, error)

	// @GetRequest("/groups")
	Groups(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]ContactGroup, error)







}
