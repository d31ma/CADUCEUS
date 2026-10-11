// UsersClient is the /users route group. The shell reads and saves the
// signed-in user's preferences; the composer also reads the sender aliases
// and saves its grammar choices.
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

	// @GetRequest("/aliases")
	Aliases(
		// @RequestHeader("Authorization")
		authorization string,
	) (AliasList, error)

	// @PutRequest("/me/preferences")
	SavePreferences(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		patch BehaviorValue,
	) (UserPreferences, error)
}
