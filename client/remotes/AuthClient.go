// @HTTPClient("/auth")
type AuthClient interface {
	// @PostRequest("/credentials/passkey/begin")
	BeginPasskey(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (PasskeyProvision, error)

	// @PostRequest("/credentials/passkey/finish")
	FinishPasskey(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @DeleteRequest("/credentials/passkey")
	RemovePasskey(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/credentials/totp/enroll")
	EnrollAuthenticator(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (AuthenticatorEnrollment, error)

	// @PostRequest("/credentials/totp/activate")
	ActivateAuthenticator(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/credentials/passkey")
	Passkeys(
		// @RequestHeader("Authorization")
		authorization string,
	) (PasskeyList, error)

	// @PutRequest("/credentials/password")
	SetPassword(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/credentials/recovery")
	ReplaceRecoveryCodes(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (RecoveryCodeSet, error)

	// @PostRequest("/login/begin")
	BeginLogin(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (LoginTransaction, error)

	// @PostRequest("/login/password")
	LoginPassword(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/login/recovery")
	LoginRecovery(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		body BehaviorValue,
	) (LoginVerification, error)
}
