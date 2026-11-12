package oidc

import (
	"errors"
	"testing"
)

func passkeyOriginsRefused(t *testing.T, text string) {
	t.Helper()
	if !errors.Is(PasskeyOrigins{Text: text}.Problem(), NotConfigured{}) {
		t.Fatalf("want %q refused", text)
	}
}

func TestPasskeyOriginsTakeTheRelyingPartyFromTheFirstHttpsOrigin(t *testing.T) {
	origins := PasskeyOrigins{Text: " android:apk-key-hash:abc  https://mail.example.com:8443 https://mail.example.com"}
	realmEqual(t, "relying party", origins.RelyingPartyId(), "mail.example.com")
	realmEqual(t, "field", origins.Field(), "android:apk-key-hash:abc https://mail.example.com:8443 https://mail.example.com")
	realmIs(t, origins.Problem(), nil)
}

func TestPasskeyOriginsUseLocalhostWhenOnlyPlainLocalhostIsListed(t *testing.T) {
	origins := PasskeyOrigins{Text: "http://localhost:8080"}
	realmEqual(t, "relying party", origins.RelyingPartyId(), "localhost")
	realmIs(t, origins.Problem(), nil)
}

func TestPasskeyOriginsAllowPlainLocalhostNextToHttpsLocalhost(t *testing.T) {
	realmIs(t, PasskeyOrigins{Text: "https://localhost http://localhost:8080"}.Problem(), nil)
}

func TestPasskeyOriginsRefuseAnHttpsOriginWithAnotherHost(t *testing.T) {
	passkeyOriginsRefused(t, "https://mail.example.com https://other.test")
}

func TestPasskeyOriginsRefuseAPlainOriginThatIsNotLocalhost(t *testing.T) {
	passkeyOriginsRefused(t, "https://localhost http://127.0.0.1:8080")
	passkeyOriginsRefused(t, "http://mail.example.com")
}

func TestPasskeyOriginsRefuseAListWithoutAWebOrigin(t *testing.T) {
	passkeyOriginsRefused(t, "android:apk-key-hash:abc")
	passkeyOriginsRefused(t, "")
}

func TestPasskeyRecordAnswersAnEmptyLastUsedBeforeTheFirstSignIn(t *testing.T) {
	realmEqual(t, "passkey", passkeyRecord("passkey-1", "Mac").Passkey(), Passkey{PasskeyId: "passkey-1", Label: "Mac", CreatedAt: "2025-11-12T00:00:00Z"})
}

func TestPasskeyAccountIsUnavailableWithoutOrigins(t *testing.T) {
	realm := newMesailRealm(t)
	account := realm.account(t)
	account.Session.Sessions.Settings.Origins = ""
	_, err := account.Passkeys().Setup(realmCaller(), "right-password", "")
	realmIs(t, err, SessionUnavailable{})
	_, challengeErr := account.Session.PasskeyChallenge()
	realmIs(t, challengeErr, SessionUnavailable{})
}
