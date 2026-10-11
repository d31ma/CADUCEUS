// UsersClient is the /users route group. The shell reads and saves the
// signed-in user's preferences.
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

	// SavePreferences saves some of the signed-in user's preferences.
	//
	// @PutRequest("/me/preferences")
	SavePreferences(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		preferences BehaviorValue,
	) (UserPreferences, error)
}
