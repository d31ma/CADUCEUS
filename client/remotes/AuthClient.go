// AuthClient is the authentication route group: the sign-in transaction, the
// session token set and the signed-in credential changes.
// @HTTPClient("/auth")
type AuthClient interface {
	// @PostRequest("/login/begin")
	Begin(
		// @RequestBody
		start LoginStart,
	) (LoginState, error)

	// @PostRequest("/login/password")
	Password(
		// @RequestBody
		factor PasswordFactor,
	) (LoginState, error)

	// @PostRequest("/login/totp")
	AuthenticatorCode(
		// @RequestBody
		factor CodeFactor,
	) (LoginState, error)

	// @PostRequest("/login/recovery")
	Recovery(
		// @RequestBody
		factor CodeFactor,
	) (LoginState, error)

	// @PostRequest("/login/passkey/options")
	PasskeyOptions(
		// @RequestBody
		transaction LoginTransaction,
	) (PasskeyOptions, error)

	// @PostRequest("/login/passkey")
	Passkey(
		// @RequestBody
		assertion PasskeyAssertion,
	) (LoginState, error)

	// @PostRequest("/login/complete")
	Complete(
		// @RequestBody
		transaction LoginTransaction,
	) (TokenSet, error)

	// @PostRequest("/logout")
	Logout(
		// @RequestBody
		logout LogoutRequest,
	) (BehaviorValue, error)

	// @PostRequest("/token/refresh")
	Refresh(
		// @RequestBody
		refresh RefreshRequest,
	) (TokenSet, error)

	// @GetRequest("/session")
	Session(
		// @RequestHeader("Authorization")
		authorization string,
	) (BehaviorValue, error)

	// @PutRequest("/credentials/password")
	ChangePassword(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		change CredentialChange,
	) (BehaviorValue, error)

	// @PostRequest("/credentials/passkey/begin")
	BeginPasskeyEnrollment(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		change CredentialChange,
	) (PasskeyEnrollment, error)

	// @PostRequest("/credentials/passkey/finish")
	FinishPasskeyEnrollment(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		attestation PasskeyAttestation,
	) (BehaviorValue, error)
}
