import "errors"

// @Test(GatewayTestSetup, {"partition":"tests"})
type CallKeysControllerTest struct {
	behaviors *BehaviorContext
}

func (test *CallKeysControllerTest) TestGetKeysServesTheCallKeyAsAJsonWebKey() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	setup.UseSignallingSettings(realm, "http://127.0.0.1:1", setup.CallKey())
	keys, failure := test.controller().GetKeys()
	Assert.IsTrue(failure == nil, "the keys failed")
	Assert.Equals(1, len(keys.Keys))
	Assert.Equals("EC", keys.Keys[0].KeyType)
	Assert.Equals("P-256", keys.Keys[0].Curve)
	Assert.Equals("ES256", keys.Keys[0].Algorithm)
}

func (test *CallKeysControllerTest) TestGetKeysWithoutAKeyIsUnavailable() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	setup.UseSignallingSettings(realm, "http://127.0.0.1:1", "")
	_, failure := test.controller().GetKeys()
	unavailable := &Unavailable{}
	Assert.IsTrue(errors.As(failure, &unavailable), "keys without a key were served")
	Assert.Equals(5042, unavailable.Number())
}

func (test *CallKeysControllerTest) controller() *CallKeysController {
	return &CallKeysController{keys: &CallKeysService{failures: &ErrorMappingService{}}}
}
