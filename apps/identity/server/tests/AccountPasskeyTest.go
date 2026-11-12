import (
	"errors"
	"strings"
)

// @Test(OneRealmTestSetup, {"partition":"tests"})
type AccountPasskeyTest struct {
	behaviors *BehaviorContext
}

func (test *AccountPasskeyTest) signedIn() string {
	OneRealmTestSetup{}.Start()
	session, err := OneRealmTestSetup{}.Account().PostSession(SignInRequest{Login: "ana", Password: "right"})
	if err != nil {
		panic(err)
	}
	return "Bearer " + session.AccessToken
}

func (test *AccountPasskeyTest) TestPostChallengeAnswersRequestOptionsForTheInstallHost() {
	OneRealmTestSetup{}.Start()
	challenge, err := OneRealmTestSetup{}.Account().PostPasskeyChallenge()
	Assert.IsTrue(err == nil)
	Assert.IsTrue(strings.Contains(challenge.OptionsJson, `"rpId":"mail.mesail.test"`))
	Assert.IsTrue(strings.Contains(challenge.OptionsJson, `"userVerification":"required"`))
	Assert.IsTrue(challenge.ChallengeKey != "")
}

func (test *AccountPasskeyTest) TestPostSessionSignsInWithAPasskey() {
	OneRealmTestSetup{}.Start()
	challenge, _ := OneRealmTestSetup{}.Account().PostPasskeyChallenge()
	session, err := OneRealmTestSetup{}.Account().PostPasskeySession(PasskeySignIn{ChallengeKey: challenge.ChallengeKey, CredentialJson: "good-assertion"})
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", session.Me.MemberId)
	Assert.Equals("example.com", session.Me.DomainId)
}

func (test *AccountPasskeyTest) TestPostSessionAnswersUnauthenticatedForAWrongPasskey() {
	OneRealmTestSetup{}.Start()
	challenge, _ := OneRealmTestSetup{}.Account().PostPasskeyChallenge()
	_, err := OneRealmTestSetup{}.Account().PostPasskeySession(PasskeySignIn{ChallengeKey: challenge.ChallengeKey, CredentialJson: "bad-assertion"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
	Assert.Equals(501, unauthenticated.Number())
}

func (test *AccountPasskeyTest) TestPostSessionAnswersNotAMemberForAnotherDomain() {
	OneRealmTestSetup{}.Start()
	challenge, _ := OneRealmTestSetup{}.Account().PostPasskeyChallenge()
	_, err := OneRealmTestSetup{}.Account().PostPasskeySession(PasskeySignIn{ChallengeKey: challenge.ChallengeKey, CredentialJson: "good-assertion", DomainId: "example.net"})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden))
	Assert.Equals(505, forbidden.Number())
	Assert.Equals("not_a_member", forbidden.Code())
}

func (test *AccountPasskeyTest) TestGetPasskeysHidesTheCredentialId() {
	passkeys, err := OneRealmTestSetup{}.Account().GetPasskeys(test.signedIn())
	Assert.IsTrue(err == nil)
	Assert.Equals(1, len(passkeys))
	Assert.Equals("Mac", passkeys[0].Label)
	Assert.Equals("2025-11-12T00:00:00Z", passkeys[0].CreatedAt)
	Assert.Equals("", passkeys[0].LastUsedAt)
}

func (test *AccountPasskeyTest) TestPostSetupAnswersCreationOptions() {
	setup, err := OneRealmTestSetup{}.Account().PostPasskeySetup(test.signedIn(), PasskeySetupRequest{CurrentPassword: "right"})
	Assert.IsTrue(err == nil)
	Assert.IsTrue(strings.Contains(setup.OptionsJson, `"excludeCredentials":[{"type":"public-key","id":"credential-1"}]`))
	Assert.IsTrue(setup.SetupKey != "")
}

func (test *AccountPasskeyTest) TestPostSetupRefusesAWrongPassword() {
	_, err := OneRealmTestSetup{}.Account().PostPasskeySetup(test.signedIn(), PasskeySetupRequest{CurrentPassword: "wrong"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
	Assert.Equals(501, unauthenticated.Number())
}

func (test *AccountPasskeyTest) TestPostPasskeyNumbersATakenLabel() {
	authorization := test.signedIn()
	setup, _ := OneRealmTestSetup{}.Account().PostPasskeySetup(authorization, PasskeySetupRequest{CurrentPassword: "right"})
	added, err := OneRealmTestSetup{}.Account().PostPasskey(authorization, PasskeyConfirm{SetupKey: setup.SetupKey, CredentialJson: "good-credential", Label: "Mac"})
	Assert.IsTrue(err == nil)
	Assert.Equals("Mac 2", added.Label)
}

func (test *AccountPasskeyTest) TestPostPasskeyAnswersInvalidPasskey() {
	authorization := test.signedIn()
	setup, _ := OneRealmTestSetup{}.Account().PostPasskeySetup(authorization, PasskeySetupRequest{CurrentPassword: "right"})
	_, err := OneRealmTestSetup{}.Account().PostPasskey(authorization, PasskeyConfirm{SetupKey: setup.SetupKey, CredentialJson: "bad-credential", Label: "Phone"})
	var bad *BadRequest
	Assert.IsTrue(errors.As(err, &bad))
	Assert.Equals(513, bad.Number())
	Assert.Equals("invalid_passkey", bad.Code())
}

func (test *AccountPasskeyTest) TestPatchPasskeyRenames() {
	renamed, err := OneRealmTestSetup{}.Account().PatchPasskey(test.signedIn(), "passkey-1", PasskeyRename{Label: "Work laptop"})
	Assert.IsTrue(err == nil)
	Assert.Equals("Work laptop", renamed.Label)
}

func (test *AccountPasskeyTest) TestPatchPasskeyRefusesAnEmptyLabel() {
	_, err := OneRealmTestSetup{}.Account().PatchPasskey(test.signedIn(), "passkey-1", PasskeyRename{Label: " "})
	var invalid *BadRequest
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals(514, invalid.Number())
}

func (test *AccountPasskeyTest) TestDeletePasskeyAnswersAFreshSession() {
	session, err := OneRealmTestSetup{}.Account().DeletePasskey(test.signedIn(), "passkey-1", PasskeyRemove{CurrentPassword: "right"})
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", session.Me.MemberId)
	Assert.IsTrue(session.SessionKey != "")
}

func (test *AccountPasskeyTest) TestDeletePasskeyAnswersNotFoundForAnotherPasskey() {
	_, err := OneRealmTestSetup{}.Account().DeletePasskey(test.signedIn(), "passkey-9", PasskeyRemove{CurrentPassword: "right"})
	var notFound *NotFound
	Assert.IsTrue(errors.As(err, &notFound))
	Assert.Equals(515, notFound.Number())
}
