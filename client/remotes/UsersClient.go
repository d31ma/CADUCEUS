// @HTTPClient
type UsersClient interface {
	// @GetRequest("/users/me/preferences")
	Preferences(
		// @RequestHeader("Authorization")
		authorization string,
	) (ImagePreferences, error)

	// @PutRequest("/users/me/preferences")
	SavePreferences(
		// @RequestBody
		preferences BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (ImagePreferences, error)
}
