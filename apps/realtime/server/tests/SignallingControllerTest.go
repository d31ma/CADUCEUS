import (
	"encoding/json"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Test(GatewayTestSetup, {"partition":"tests"})
type SignallingControllerTest struct {
	behaviors *BehaviorContext
}

func (test *SignallingControllerTest) TestMembersExchangeSignalsThroughTheGateway() {
	frames := make(chan string, 32)
	controller := test.signedIn(frames)
	_, failure := controller.PostMessage(GatewayMessage{ConnectionId: "socket-1", DomainId: "example.com", Body: `{"type":"signal","requestId":"r1","toDeviceIds":["device-2"],"kind":"rtc","ciphertext":"b2ZmZXI"}`})
	Assert.IsTrue(failure == nil, "the signal failed")
	Assert.Equals(`socket-2 {"type":"signal","fromDeviceId":"device-1","fromPersonId":"member-1","kind":"rtc","ciphertext":"b2ZmZXI"}`, GatewayTestSetup{}.Next(frames))
	Assert.Equals(`socket-1 {"type":"signal.result","requestId":"r1","delivered":["device-2"],"offline":[]}`, GatewayTestSetup{}.Next(frames))
}

func (test *SignallingControllerTest) TestASignalToADeviceThatIsNotAMembersIsRefused() {
	frames := make(chan string, 32)
	controller := test.signedIn(frames)
	_, failure := controller.PostMessage(GatewayMessage{ConnectionId: "socket-1", DomainId: "example.com", Body: `{"type":"signal","toDeviceIds":["device-of-a-stranger"],"kind":"rtc","ciphertext":"b2ZmZXI"}`})
	Assert.IsTrue(failure == nil, "the refusal failed")
	Assert.Equals(`socket-1 {"type":"refused","requestType":"signal","state":"not_found","field":"toDeviceIds","message":"[MS5031]: Nothing was found."}`, GatewayTestSetup{}.Next(frames))
}

func (test *SignallingControllerTest) TestAGroupCallTokenIsSignedByAPublishedKey() {
	frames := make(chan string, 32)
	controller := test.signedIn(frames)
	_, failure := controller.PostMessage(GatewayMessage{ConnectionId: "socket-1", DomainId: "example.com", Body: `{"type":"call.create","media":"video","group":true}`})
	Assert.IsTrue(failure == nil, "creating the call failed")
	created := map[string]string{}
	json.Unmarshal([]byte(strings.TrimPrefix(GatewayTestSetup{}.Next(frames), "socket-1 ")), &created)
	token, parseFailure := oidc.TokenParser{}.Parse(created["joinToken"])
	keys, keysFailure := (&CallKeysController{keys: &CallKeysService{failures: &ErrorMappingService{}}}).GetKeys()
	Assert.IsTrue(parseFailure == nil && keysFailure == nil, "the token or the keys failed")
	Assert.Equals("call.created", created["type"])
	Assert.Equals(token.Header.KeyId, keys.Keys[0].KeyId)
	key := oidc.JsonWebKey{KeyType: keys.Keys[0].KeyType, KeyId: keys.Keys[0].KeyId, Curve: keys.Keys[0].Curve, X: keys.Keys[0].X, Y: keys.Keys[0].Y}
	Assert.IsTrue((oidc.SignatureCheck{}).Verify(token, key) == nil, "the join token does not verify with the published key")
}

func (test *SignallingControllerTest) signedIn(frames chan string) *GatewayController {
	setup := GatewayTestSetup{}
	realm, _ := setup.Install(frames)
	controller := &GatewayController{gateway: &GatewayService{failures: &ErrorMappingService{}}}
	controller.PostMessage(GatewayMessage{ConnectionId: "socket-1", Body: `{"type":"auth","token":"` + setup.MemberToken(realm, "member-1") + `","deviceId":"device-1"}`})
	controller.PostMessage(GatewayMessage{ConnectionId: "socket-2", Body: `{"type":"auth","token":"` + setup.MemberToken(realm, "member-2") + `","deviceId":"device-2"}`})
	return controller
}
