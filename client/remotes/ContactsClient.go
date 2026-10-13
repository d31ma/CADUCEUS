// ContactsClient is the /contacts route group: the contact list, its groups,
// the CardDAV address books and their queued changes, vCard import and
// export, and the composer's address suggestions.
// @HTTPClient({"path":"/contacts","setup":"SameOriginSetup"})
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

	// @PostRequest("/groups")
	CreateGroup(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		group BehaviorValue,
	) (ContactGroup, error)

	// @PutRequest("/groups/{id}")
	RenameGroup(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
		// @RequestBody
		group BehaviorValue,
	) (ContactGroup, error)

	// @DeleteRequest("/groups/{id}")
	DeleteGroup(
		// @RequestHeader("Authorization")
		authorization string,
		// @PathVariable
		id string,
	) (ContactDeletion, error)

	// @GetRequest("/address-books")
	AddressBooks(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]AddressBook, error)

	// @PostRequest("/import")
	Import(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		vcard BehaviorValue,
	) (ContactImport, error)

	// @PostRequest("/export")
	Export(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		selection BehaviorValue,
	) (ContactExport, error)

	// @GetRequest("/suggestions")
	Suggestions(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestParam("q")
		query string,
	) ([]ContactSuggestion, error)
}
