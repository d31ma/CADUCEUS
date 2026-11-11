import (
	"errors"
	"strings"
	"time"

	"mesail.dev/packages/oidc"
)

// @Test(KeycloakTestSetup, {"partition":"tests"})
type AccountControllerTest struct {
	behaviors *BehaviorContext
}

func (test *AccountControllerTest) signedIn() string {
	setup := KeycloakTestSetup{}
	realm := setup.Realm("example.com")
	setup.Keycloak(realm)
	return "Bearer " + setup.Token(realm, setup.Member())
}

func (test *AccountControllerTest) TestPostPasswordAnswersAFreshSession() {
	authorization := test.signedIn()
	session, err := (&AccountController{}).PostPassword(authorization, PasswordChange{CurrentPassword: "right", NewPassword: "a-new-long-password"})
	Assert.IsTrue(err == nil)
	Assert.Equals("member-1", session.Me.MemberId)
	Assert.IsTrue(session.SessionKey != "")
}

func (test *AccountControllerTest) TestPostPasswordNamesTheFailedPolicy() {
	authorization := test.signedIn()
	_, err := (&AccountController{}).PostPassword(authorization, PasswordChange{CurrentPassword: "right", NewPassword: "short"})
	var policy *PasswordPolicy
	Assert.IsTrue(errors.As(err, &policy))
	Assert.Equals("Invalid password: minimum length 12.", policy.Rule)
}

func (test *AccountControllerTest) TestPostPasswordRefusesAWrongCurrentPassword() {
	authorization := test.signedIn()
	_, err := (&AccountController{}).PostPassword(authorization, PasswordChange{CurrentPassword: "wrong", NewPassword: "a-new-long-password"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *AccountControllerTest) TestGetTotpIsOffWithoutACredential() {
	status, err := (&AccountController{}).GetTotp(test.signedIn())
	Assert.IsTrue(err == nil)
	Assert.IsTrue(!status.Enabled)
}

func (test *AccountControllerTest) TestPostTotpSetupAnswersTheQrImage() {
	setup, err := (&AccountController{}).PostTotpSetup(test.signedIn(), TotpSetupRequest{CurrentPassword: "right"})
	Assert.IsTrue(err == nil)
	Assert.IsTrue(strings.HasPrefix(setup.QrImage, "data:image/png;base64,"))
	Assert.IsTrue(strings.HasPrefix(setup.OtpauthUri, "otpauth://totp/"))
	Assert.IsTrue(setup.SetupKey != "")
}

func (test *AccountControllerTest) TestPostTotpConfirmStoresARightCode() {
	authorization := test.signedIn()
	setup, _ := (&AccountController{}).PostTotpSetup(authorization, TotpSetupRequest{CurrentPassword: "right"})
	code, _ := oidc.Totp{}.Code(setup.Secret, time.Now())
	Assert.IsTrue((&AccountController{}).PostTotpConfirm(authorization, TotpConfirm{SetupKey: setup.SetupKey, Code: code}) == nil)
}

func (test *AccountControllerTest) TestPostTotpConfirmRefusesAWrongCode() {
	authorization := test.signedIn()
	setup, _ := (&AccountController{}).PostTotpSetup(authorization, TotpSetupRequest{CurrentPassword: "right"})
	err := (&AccountController{}).PostTotpConfirm(authorization, TotpConfirm{SetupKey: setup.SetupKey, Code: "000000"})
	var invalid *InvalidCode
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *AccountControllerTest) TestDeleteTotpChecksThePassword() {
	authorization := test.signedIn()
	Assert.IsTrue((&AccountController{}).DeleteTotp(authorization, TotpRemove{CurrentPassword: "right", OneTimeCode: "123456"}) == nil)
	err := (&AccountController{}).DeleteTotp(authorization, TotpRemove{CurrentPassword: "wrong", OneTimeCode: "123456"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *AccountControllerTest) TestAccountRoutesAreUnavailableWithoutSettings() {
	KeycloakTestSetup{}.Unconfigured()
	_, err := (&AccountController{}).GetTotp("Bearer x")
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable))
}
