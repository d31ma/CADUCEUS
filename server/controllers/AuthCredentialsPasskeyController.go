import "net/http"

// AuthCredentialsPasskeyController answers /auth/credentials/passkey: the
// caller's own registered passkeys. The boundary refuses a credential that
// belongs to anyone else.
// @RequestRoute("/auth/credentials/passkey")
// @Public
type AuthCredentialsPasskeyController struct{}

// GetPasskeys answers the caller's registered passkeys.
// @GetRoute
func (*AuthCredentialsPasskeyController) GetPasskeys(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.passkey.list", Signed: true}.Answer(request)
}

// DeletePasskey unregisters one of the caller's passkeys after a freshly
// satisfied authentication transaction.
// @DeleteRoute
func (*AuthCredentialsPasskeyController) DeletePasskey(request *http.Request) *http.Response {
	return AuthRelay{Operation: "auth.credential.passkey.remove", Fields: []string{"stepUpTransactionId", "credentialId"}, Signed: true}.Answer(request)
}
