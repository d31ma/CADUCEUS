import (
	"encoding/json"
	"time"
)

// @Test(GatewayTestSetup, {"partition":"tests"})
type WaitingEnvelopesTest struct {
	behaviors *BehaviorContext
}

func (test *WaitingEnvelopesTest) TestAStoredEnvelopeReachesOnlyTheLinkedSocketOfAListedDevice() {
	frames := make(chan string, 32)
	setup := GatewayTestSetup{}
	realm, _ := setup.Install(frames)
	gateway := &GatewayController{gateway: &GatewayService{failures: &ErrorMappingService{}}}
	gateway.PostMessage(GatewayMessage{ConnectionId: "waiting-linked", Body: `{"type":"auth","token":"` + setup.MemberToken(realm, "member-w1") + `","deviceId":"device-w1"}`})
	gateway.PostMessage(GatewayMessage{ConnectionId: "waiting-other-device", Body: `{"type":"auth","token":"` + setup.MemberToken(realm, "member-w1") + `","deviceId":"device-w3"}`})
	gateway.PostMessage(GatewayMessage{ConnectionId: "waiting-no-device", Body: `{"type":"auth","token":"` + setup.MemberToken(realm, "member-w1") + `"}`})
	gateway.PostMessage(GatewayMessage{ConnectionId: "waiting-other-person", Body: `{"type":"auth","token":"` + setup.MemberToken(realm, "member-w2") + `","deviceId":"device-w2"}`})
	payload, _ := json.Marshal(map[string]any{"toPersonId": "member-w1", "toDeviceIds": []string{"device-w1", "device-w2", "device-w3"}, "kind": "message", "envelopeId": "env_secret"})
	controller := &EventsController{events: &EventService{failures: &ErrorMappingService{}}}
	envelope := EventEnvelope{EventId: "evt_waiting_" + time.Now().Format("150405.000000000"), Type: "envelope.stored", DomainId: "example.com", Producer: "devicebox", OccurredAt: time.Now().UTC().Format(time.RFC3339), Payload: payload}
	receipt, failure := controller.PostEvent("Bearer "+setup.ServiceToken(realm), envelope)
	Assert.IsTrue(failure == nil, "the event was refused")
	Assert.Equals("applied", receipt.Status)
	Assert.Equals(`waiting-linked {"type":"devicebox.waiting","conversationId":"","messageId":"","state":"message"}`, setup.Next(frames))
	Assert.Equals(0, len(frames))
}
