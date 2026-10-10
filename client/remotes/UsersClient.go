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
		change BehaviorValue,
	) (AliasList, error)

	// @DeleteRequest("/users/aliases")
	RemoveAlias(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		change BehaviorValue,
	) (AliasList, error)
}
