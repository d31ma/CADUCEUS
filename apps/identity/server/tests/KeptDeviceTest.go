import (
	"errors"
	"os"
	"time"
)

// @Test(OneRealmTestSetup, {"partition":"tests"})
type KeptDeviceTest struct {
	behaviors *BehaviorContext
}

func (test *KeptDeviceTest) signedIn(kept bool) Session {
	OneRealmTestSetup{}.Start()
	session, err := OneRealmTestSetup{}.Account().PostSession(SignInRequest{Login: "ana", Password: "right", KeepSignedIn: kept})
	if err != nil {
		panic(err)
	}
	return session
}

func (test *KeptDeviceTest) TestPostSessionIsKeptOnlyWhenAsked() {
	Assert.IsTrue(test.signedIn(true).Kept)
	Assert.IsTrue(!test.signedIn(false).Kept)
}

func (test *KeptDeviceTest) TestRefreshAndSwitchKeepTheKeptFlag() {
	session := test.signedIn(true)
	refreshed, refreshErr := OneRealmTestSetup{}.Account().PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	switched, switchErr := OneRealmTestSetup{}.Account().PostDomain(DomainSwitch{SessionKey: refreshed.SessionKey, DomainId: "example.org"})
	Assert.IsTrue(refreshErr == nil && switchErr == nil)
	Assert.IsTrue(refreshed.Kept)
	Assert.IsTrue(switched.Kept)
}

func (test *KeptDeviceTest) TestRefreshOfASessionEndedElsewhereIsRevoked() {
	session := test.signedIn(true)
	os.Setenv(OneRealmTestSetup{}.EndedVariable(), time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano))
	_, err := OneRealmTestSetup{}.Account().PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden))
	Assert.Equals("access_revoked", forbidden.Code())
}

func (test *KeptDeviceTest) TestRefreshOfASessionStartedAfterTheEndWorks() {
	session := test.signedIn(true)
	os.Setenv(OneRealmTestSetup{}.EndedVariable(), time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano))
	_, err := OneRealmTestSetup{}.Account().PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	Assert.IsTrue(err == nil)
}

func (test *KeptDeviceTest) TestDeleteSessionsSignsOutOnAllDevices() {
	session := test.signedIn(true)
	err := OneRealmTestSetup{}.Account().DeleteSessions("Bearer " + session.AccessToken)
	Assert.IsTrue(err == nil)
}

func (test *KeptDeviceTest) TestDeleteSessionsNeedsASignedInCaller() {
	test.signedIn(true)
	err := OneRealmTestSetup{}.Account().DeleteSessions("")
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *KeptDeviceTest) TestPostPasswordKeepsTheDeviceWhenAsked() {
	session := test.signedIn(true)
	fresh, err := OneRealmTestSetup{}.Account().PostPassword("Bearer "+session.AccessToken, PasswordChange{CurrentPassword: "right", NewPassword: "a-new-long-password", KeepSignedIn: true})
	Assert.IsTrue(err == nil)
	Assert.IsTrue(fresh.Kept)
}

func (test *KeptDeviceTest) TestDeletePasskeyKeepsTheDeviceWhenAsked() {
	session := test.signedIn(true)
	fresh, err := OneRealmTestSetup{}.Account().DeletePasskey("Bearer "+session.AccessToken, "passkey-1", PasskeyRemove{CurrentPassword: "right", KeepSignedIn: true})
	Assert.IsTrue(err == nil)
	Assert.IsTrue(fresh.Kept)
}
