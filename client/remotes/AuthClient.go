// AuthClient is the authentication route group: the sign-in transaction, the
// session token set and the signed-in credential changes.
// @HTTPClient("/auth")
type AuthClient interface {
	// @PostRequest("/login/begin")
	Begin(
		// @RequestBody
		start BehaviorValue,
	) (LoginState, error)

	// @PostRequest("/login/password")
	Password(
		// @RequestBody
		factor BehaviorValue,
	) (LoginState, error)

	// @PostRequest("/login/totp")
	AuthenticatorCode(
		// @RequestBody
		factor BehaviorValue,
	) (LoginState, error)

	// @PostRequest("/login/recovery")
	Recovery(
		// @RequestBody
		factor BehaviorValue,
	) (LoginState, error)

	// @PostRequest("/login/passkey/options")
	PasskeyOptions(
		// @RequestBody
		transaction BehaviorValue,
	) (PasskeyOptions, error)

	// @PostRequest("/login/passkey")
	Passkey(
		// @RequestBody
		assertion BehaviorValue,
	) (LoginState, error)

	// @PostRequest("/login/complete")
	Complete(
		// @RequestBody
		transaction BehaviorValue,
	) (TokenSet, error)

	// @PostRequest("/logout")
	Logout(
		// @RequestBody
		logout BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/token/refresh")
	Refresh(
		// @RequestBody
		refresh BehaviorValue,
	) (TokenSet, error)

	// @GetRequest("/session")
	Session(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/credentials/password")
	ChangePassword(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		change BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/credentials/passkey/begin")
	BeginPasskeyEnrollment(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		change BehaviorValue,
	) (PasskeyEnrollment, error)

	// @PostRequest("/credentials/passkey/finish")
	FinishPasskeyEnrollment(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		attestation BehaviorValue,
	) (BehaviorValue, error)
}
