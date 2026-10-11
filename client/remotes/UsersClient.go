// UsersClient is the /users route group. The shell reads and saves the
// signed-in user's preferences; settings also saves the other preferences,
// manages the user's aliases and lets an administrator add users.
// @HTTPClient("/users")
type UsersClient interface {
	// @GetRequest("/me/preferences")
	Preferences(
		// @RequestHeader("Authorization")
		authorization string,
	) (UserPreferences, error)

	// @PutRequest("/me/preferences")
	SaveTheme(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		theme BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/me/preferences")
	SavePreferences(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		patch BehaviorValue,
	) (UserPreferences, error)

	// @GetRequest("/aliases")
	Aliases(
		// @RequestHeader("Authorization")
		authorization string,
	) (AliasList, error)

	// @PostRequest("/aliases")
	AddAlias(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		alias BehaviorValue,
	) (AliasList, error)

	// @DeleteRequest("/aliases")
	RemoveAlias(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		alias BehaviorValue,
	) (AliasList, error)

	// @GetRequest
	Users(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]UserAccount, error)

	// @PostRequest
	AddUser(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		user BehaviorValue,
	) (BehaviorValue, error)
}
