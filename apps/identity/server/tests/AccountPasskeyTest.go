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
	session, err := (&AccountController{}).PostSession(SignInRequest{Login: "ana", Password: "right"})
	if err != nil {
		panic(err)
	}
	return "Bearer " + session.AccessToken
}

func (test *AccountPasskeyTest) TestPostChallengeAnswersRequestOptionsForTheInstallHost() {
	OneRealmTestSetup{}.Start()
	challenge, err := (&AccountController{}).PostPasskeyChallenge()
	Assert.IsTrue(err == nil)
	Assert.IsTrue(strings.Contains(challenge.OptionsJson, `"rpId":"mail.mesail.test"`))
	Assert.IsTrue(strings.Contains(challenge.OptionsJson, `"userVerification":"required"`))
	Assert.IsTrue(challenge.ChallengeKey != "")
}

func (test *AccountPasskeyTest) TestPostSessionSignsInWithAPasskey() {
	OneRealmTestSetup{}.Start()
	challenge, _ := (&AccountController{}).PostPasskeyChallenge()
	session, err := (&AccountController{}).PostPasskeySession(PasskeySignIn{ChallengeKey: challenge.ChallengeKey, CredentialJson: "good-assertion"})
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", session.Me.MemberId)
	Assert.Equals("example.com", session.Me.DomainId)
}

func (test *AccountPasskeyTest) TestPostSessionAnswersUnauthenticatedForAWrongPasskey() {
	OneRealmTestSetup{}.Start()
	challenge, _ := (&AccountController{}).PostPasskeyChallenge()
	_, err := (&AccountController{}).PostPasskeySession(PasskeySignIn{ChallengeKey: challenge.ChallengeKey, CredentialJson: "bad-assertion"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *AccountPasskeyTest) TestPostSessionAnswersNotAMemberForAnotherDomain() {
	OneRealmTestSetup{}.Start()
	challenge, _ := (&AccountController{}).PostPasskeyChallenge()
	_, err := (&AccountController{}).PostPasskeySession(PasskeySignIn{ChallengeKey: challenge.ChallengeKey, CredentialJson: "good-assertion", DomainId: "example.net"})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden))
	Assert.Equals("not_a_member", forbidden.Code)
}

func (test *AccountPasskeyTest) TestGetPasskeysHidesTheCredentialId() {
	passkeys, err := (&AccountController{}).GetPasskeys(test.signedIn())
	Assert.IsTrue(err == nil)
	Assert.Equals(1, len(passkeys))
	Assert.Equals("Mac", passkeys[0].Label)
	Assert.Equals("2025-11-12T00:00:00Z", passkeys[0].CreatedAt)
	Assert.Equals("", passkeys[0].LastUsedAt)
}

func (test *AccountPasskeyTest) TestPostSetupAnswersCreationOptions() {
	setup, err := (&AccountController{}).PostPasskeySetup(test.signedIn(), PasskeySetupRequest{CurrentPassword: "right"})
	Assert.IsTrue(err == nil)
	Assert.IsTrue(strings.Contains(setup.OptionsJson, `"excludeCredentials":[{"type":"public-key","id":"credential-1"}]`))
	Assert.IsTrue(setup.SetupKey != "")
}

func (test *AccountPasskeyTest) TestPostSetupRefusesAWrongPassword() {
	_, err := (&AccountController{}).PostPasskeySetup(test.signedIn(), PasskeySetupRequest{CurrentPassword: "wrong"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *AccountPasskeyTest) TestPostPasskeyNumbersATakenLabel() {
	authorization := test.signedIn()
	setup, _ := (&AccountController{}).PostPasskeySetup(authorization, PasskeySetupRequest{CurrentPassword: "right"})
	added, err := (&AccountController{}).PostPasskey(authorization, PasskeyConfirm{SetupKey: setup.SetupKey, CredentialJson: "good-credential", Label: "Mac"})
	Assert.IsTrue(err == nil)
	Assert.Equals("Mac 2", added.Label)
}

func (test *AccountPasskeyTest) TestPostPasskeyAnswersInvalidPasskey() {
	authorization := test.signedIn()
	setup, _ := (&AccountController{}).PostPasskeySetup(authorization, PasskeySetupRequest{CurrentPassword: "right"})
	_, err := (&AccountController{}).PostPasskey(authorization, PasskeyConfirm{SetupKey: setup.SetupKey, CredentialJson: "bad-credential", Label: "Phone"})
	var bad *BadPasskey
	Assert.IsTrue(errors.As(err, &bad))
	Assert.Equals("invalid_passkey", bad.Code)
}

func (test *AccountPasskeyTest) TestPatchPasskeyRenames() {
	renamed, err := (&AccountController{}).PatchPasskey(test.signedIn(), "passkey-1", PasskeyRename{Label: "Work laptop"})
	Assert.IsTrue(err == nil)
	Assert.Equals("Work laptop", renamed.Label)
}

func (test *AccountPasskeyTest) TestPatchPasskeyRefusesAnEmptyLabel() {
	_, err := (&AccountController{}).PatchPasskey(test.signedIn(), "passkey-1", PasskeyRename{Label: " "})
	var invalid *InvalidLabel
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *AccountPasskeyTest) TestDeletePasskeyAnswersAFreshSession() {
	session, err := (&AccountController{}).DeletePasskey(test.signedIn(), "passkey-1", PasskeyRemove{CurrentPassword: "right"})
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", session.Me.MemberId)
	Assert.IsTrue(session.SessionKey != "")
}

func (test *AccountPasskeyTest) TestDeletePasskeyAnswersNotFoundForAnotherPasskey() {
	_, err := (&AccountController{}).DeletePasskey(test.signedIn(), "passkey-9", PasskeyRemove{CurrentPassword: "right"})
	var notFound *NotFound
	Assert.IsTrue(errors.As(err, &notFound))
}
