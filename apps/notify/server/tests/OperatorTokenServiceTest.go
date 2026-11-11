import "os"

// @Test(NotifyTestSetup, {"partition":"tests"})
type OperatorTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *OperatorTokenServiceTest) TestTokenComesWithoutTheBearerPrefix() {
	setup := NotifyTestSetup{}
	member, operators := setup.Realms(setup.Domain())
	setup.Keycloak(member, operators)
	token, err := (&OperatorTokenService{}).Token()
	Assert.IsTrue(err == nil, "token failed")
	Assert.Equals(setup.Service(operators)[len("Bearer "):][:20], token[:20])
}

func (test *OperatorTokenServiceTest) TestTokenFailsWithoutSettings() {
	os.Setenv("YON_APPLICATION_JSON", "{}")
	_, err := (&OperatorTokenService{}).Token()
	Assert.IsTrue(err != nil, "a token came without settings")
}
