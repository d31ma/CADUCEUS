// @HTTPClient
type UsersClient interface {
	// @GetRequest("/users/aliases")
	Aliases(
		// @RequestHeader("Authorization")
		authorization string,
	) (AliasList, error)

	// @GetRequest("/users/me/preferences")
	Preferences(
		// @RequestHeader("Authorization")
		authorization string,
	) (UserPreferences, error)

	// @PutRequest("/users/me/preferences")
	UpdatePreferences(
		// @RequestBody
		patch BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)
}
