// @HTTPClient
type UsersClient interface {
	// @GetRequest("/users/aliases")
	Aliases(
		// @RequestHeader("Authorization")
		authorization string,
	) (AliasList, error)

	// @PostRequest("/users/aliases")
	AddAlias(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (AliasList, error)

	// @DeleteRequest("/users/aliases")
	RemoveAlias(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (AliasList, error)

	// @GetRequest("/users/me/preferences")
	Preferences(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/users/me/preferences")
	SavePreferences(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/users")
	Users(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]UserAccount, error)

	// @PostRequest("/users")
	AddUser(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)
}
