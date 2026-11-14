package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/oidc"
)

func callBody(frameType string, callId string) string {
	return `{"type":"` + frameType + `","requestId":"k1","callId":"` + callId + `"}`
}

func ringBody(callId string, people string) string {
	return `{"type":"call.ring","callId":"` + callId + `","toPersonIds":[` + people + `],"invite":"aW52aXRl"}`
}

func groupCall(t *testing.T, fixture signalFixture) string {
	t.Helper()
	if err := fixture.send("ca", `{"type":"call.create","requestId":"k0","media":"video","group":true}`); err != nil {
		t.Fatal(err)
	}
	return fixture.callId("ca")
}

func joinClaimsOf(t *testing.T, fixture signalFixture, token string) JoinClaims {
	t.Helper()
	parsed, err := oidc.TokenParser{}.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	set, err := fixture.gateway.Signalling.Calls.Signer.KeySet()
	if err != nil || len(set.Keys) != 1 || set.Keys[0].KeyId != parsed.Header.KeyId {
		t.Fatalf("keys %+v, %v", set, err)
	}
	if err := (oidc.SignatureCheck{}).Verify(parsed, set.Keys[0]); err != nil {
		t.Fatal(err)
	}
	payload, _ := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
	claims := JoinClaims{}
	json.Unmarshal(payload, &claims)
	return claims
}

func callFixture(t *testing.T) (signalFixture, string) {
	t.Helper()
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cb", "ben-token", "b1")
	return fixture, groupCall(t, fixture)
}

func TestCreatingAGroupCallGivesASignedJoinToken(t *testing.T) {
	fixture, callId := callFixture(t)
	frame := fixture.last("ca")
	if frame["type"] != "call.created" || frame["requestId"] != "k0" || !strings.HasPrefix(callId, "call_") {
		t.Fatalf("got %+v", frame)
	}
	claims := joinClaimsOf(t, fixture, frame["joinToken"].(string))
	issued := connectionTestTime().Unix()
	want := JoinClaims{CallId: callId, DeviceId: "a1", DomainId: "example.com", Audience: "calls", IssuedAt: issued, ExpiresAt: issued + 60}
	if claims != want {
		t.Fatalf("got %+v", claims)
	}
}

func TestATwoPersonCallHasNoJoinToken(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", `{"type":"call.create","media":"audio"}`); err != nil {
		t.Fatal(err)
	}
	frame := fixture.last("ca")
	if frame["type"] != "call.created" || frame["joinToken"] != nil {
		t.Fatalf("got %+v", frame)
	}
}

func TestAMemberJoinsAGroupCallWithTheirOwnToken(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	frame := fixture.last("cb")
	claims := joinClaimsOf(t, fixture, frame["joinToken"].(string))
	if frame["type"] != "call.joined" || frame["requestId"] != "k1" || claims.DeviceId != "b1" || claims.CallId != callId {
		t.Fatalf("got %+v, %+v", frame, claims)
	}
}

func TestJoiningTwiceKeepsOneSeat(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("ca", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	seats, _ := SeatBook{Storage: fixture.memory.storage()}.Seats("example.com", callId)
	if len(seats) != 1 || fixture.last("ca")["type"] != "call.joined" {
		t.Fatalf("seats %+v, frame %+v", seats, fixture.last("ca"))
	}
}

func TestRingingReachesOnlineMembersAndPushesTheOthers(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("ca", ringBody(callId, `"ben","cara"`)); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"type": "call.ringing", "callId": callId, "fromPersonId": "ana", "invite": "aW52aXRl"}
	if !reflect.DeepEqual(fixture.last("cb"), want) {
		t.Fatalf("ben got %+v", fixture.last("cb"))
	}
	if !reflect.DeepEqual(fixture.notify.pushes, []CallPush{{ToPersonIds: []string{"cara"}, Kind: "incoming"}}) || fixture.notify.domains[0] != "example.com" {
		t.Fatalf("pushes %+v %+v", fixture.notify.pushes, fixture.notify.domains)
	}
}

func TestRingingWhenEveryoneIsOnlineSendsNoPush(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("ca", ringBody(callId, `"ben"`)); err != nil {
		t.Fatal(err)
	}
	if len(fixture.notify.pushes) != 0 {
		t.Fatalf("pushes %+v", fixture.notify.pushes)
	}
}

func TestEndingTheRingingStopsRingingAndPushesMissedCalls(t *testing.T) {
	fixture, callId := callFixture(t)
	fixture.device("cb2", "ben-token", "b2")
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"call.ring.end","callId":"` + callId + `","missed":["cara"]}`
	if err := fixture.send("ca", body); err != nil {
		t.Fatal(err)
	}
	if fixture.last("cb2")["type"] != "call.ring.ended" || fixture.last("cb2")["callId"] != callId {
		t.Fatalf("ben's other device got %+v", fixture.last("cb2"))
	}
	if !reflect.DeepEqual(fixture.notify.pushes, []CallPush{{ToPersonIds: []string{"cara"}, Kind: "missed"}}) {
		t.Fatalf("pushes %+v", fixture.notify.pushes)
	}
}

func TestEndingTheRingingWithNobodyMissedSendsNoPush(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("ca", `{"type":"call.ring.end","callId":"`+callId+`"}`); err != nil {
		t.Fatal(err)
	}
	if len(fixture.notify.pushes) != 0 {
		t.Fatalf("pushes %+v", fixture.notify.pushes)
	}
}

func TestADeviceNotInTheCallCannotRing(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("cb", ringBody(callId, `"cara"`)); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cb", "call.ring", "not_found", "[MS5036]: ")
}

func TestRingingTooManyPeopleIsRefused(t *testing.T) {
	fixture, callId := callFixture(t)
	people := strings.TrimSuffix(strings.Repeat(`"ben",`, 50), ",")
	if err := fixture.send("ca", ringBody(callId, people)); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.ring", "invalid_request", "[MS5038]: ")
}

func TestRingingWithAnInviteOverTheLimitIsRefused(t *testing.T) {
	fixture, callId := callFixture(t)
	invite := base64.RawURLEncoding.EncodeToString(randomBytes(16385))
	if err := fixture.send("ca", `{"type":"call.ring","callId":"`+callId+`","toPersonIds":["ben"],"invite":"`+invite+`"}`); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.ring", "invalid_request", "[MS5039]: ")
}

func TestEndingTheRingingWithTooManyMissedIsRefused(t *testing.T) {
	fixture, callId := callFixture(t)
	missed := strings.TrimSuffix(strings.Repeat(`"cara",`, 50), ",")
	if err := fixture.send("ca", `{"type":"call.ring.end","callId":"`+callId+`","missed":[`+missed+`]}`); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.ring.end", "invalid_request", "[MS5040]: ")
}

func TestWhenNotifyIsDownOnlineMembersStillRing(t *testing.T) {
	fixture, callId := callFixture(t)
	fixture.notify.down = true
	if err := fixture.send("ca", ringBody(callId, `"ben","cara"`)); err != nil {
		t.Fatal(err)
	}
	if fixture.last("cb")["type"] != "call.ringing" || fixture.last("ca")["type"] == "refused" {
		t.Fatalf("ben got %+v, ana got %+v", fixture.last("cb"), fixture.last("ca"))
	}
	if !reflect.DeepEqual(fixture.pushLog.failed, []string{"example.com incoming notify is down"}) {
		t.Fatalf("logged %+v", fixture.pushLog.failed)
	}
}

func TestTheStderrLogWritesOneJsonLineWithoutTheCallContent(t *testing.T) {
	output := &strings.Builder{}
	StderrCallPushLog{Output: output}.PushFailed("example.com", "missed", errors.New("status 404"))
	line := map[string]any{}
	if err := json.Unmarshal([]byte(output.String()), &line); err != nil || line["msg"] != "realtime.call_push_failed" || line["domainId"] != "example.com" || line["kind"] != "missed" || line["error"] != "status 404" {
		t.Fatalf("got %q", output.String())
	}
}

func TestTheCallEndsWhenTheLastDeviceLeaves(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("ca", callBody("call.leave", callId)); err != nil {
		t.Fatal(err)
	}
	if fixture.last("ca")["type"] != "call.left" || fixture.last("ca")["callId"] != callId {
		t.Fatalf("got %+v", fixture.last("ca"))
	}
	if err := fixture.send("cb", callBody("call.leave", callId)); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cb", "call.join", "not_found", "[MS5035]: ")
}

func TestTheCallGoesOnWhileADeviceIsInIt(t *testing.T) {
	fixture, callId := callFixture(t)
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("ca", callBody("call.leave", callId)); err != nil {
		t.Fatal(err)
	}
	fixture.device("cc", "cara-token", "c1")
	if err := fixture.send("cc", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	if fixture.last("cc")["type"] != "call.joined" {
		t.Fatalf("got %+v", fixture.last("cc"))
	}
}

func TestTheFiftyFirstPersonCannotJoin(t *testing.T) {
	fixture, callId := callFixture(t)
	fixture.seatPeople(callId, 49)
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cb", "call.join", "call_full", "[MS5037]: ")
}

func TestAPersonInAFullCallMayAddAnotherDevice(t *testing.T) {
	fixture, callId := callFixture(t)
	fixture.seatPeople(callId, 49)
	fixture.device("ca2", "ana-token", "a2")
	if err := fixture.send("ca2", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	if fixture.last("ca2")["type"] != "call.joined" {
		t.Fatalf("got %+v", fixture.last("ca2"))
	}
}

func TestADeviceMayStartTenCallsAMinute(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	storage := fixture.memory.storage()
	starts := CallStartBook{Storage: storage, Now: fixture.clock.Now}
	if err := starts.Allow("example.com", "a1"); err != nil {
		t.Fatal(err)
	}
	record := CallStartRecord{}
	DomainShelf{Storage: storage, Base: "callstarts"}.Find("example.com", map[string]any{"deviceId": "a1"}, &record)
	record.StartedAt = append(record.StartedAt, record.StartedAt[0], record.StartedAt[0], record.StartedAt[0], record.StartedAt[0], record.StartedAt[0], record.StartedAt[0], record.StartedAt[0], record.StartedAt[0], record.StartedAt[0])
	DomainShelf{Storage: storage, Base: "callstarts"}.Patch("example.com", record.ID, CallStartRecord{DeviceId: "a1", StartedAt: record.StartedAt})
	if err := fixture.send("ca", `{"type":"call.create","media":"audio"}`); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.create", "rate_limited", "[MS5041]: ")
	fixture.clock.move(time.Minute)
	if err := fixture.send("ca", `{"type":"call.create","media":"audio"}`); err != nil {
		t.Fatal(err)
	}
	if fixture.last("ca")["type"] != "call.created" {
		t.Fatalf("got %+v", fixture.last("ca"))
	}
}

func TestAnUnlinkedDeviceCannotStartACall(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("cn", "ana-token", "new-device")
	if err := fixture.send("cn", `{"type":"call.create","media":"audio"}`); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cn", "call.create", "forbidden", "[MS5044]: ")
}

func TestACallNeedsAudioOrVideo(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", `{"type":"call.create","media":"smell"}`); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.create", "invalid_request", "[MS5034]: ")
}

func TestAGroupCallIsRefusedWithoutASigningKey(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.gateway.Signalling.Calls.Signer = CallSignerFactory{}.FromSettings(CallKeySettings{}, fixture.clock.Now)
	if err := fixture.send("ca", `{"type":"call.create","media":"video","group":true}`); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.create", "unavailable", "[MS5042]: ")
}

func TestLeavingAnUnknownCallIsNotFound(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", callBody("call.leave", "call_none")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.leave", "not_found", "[MS5035]: ")
}

func TestABadCallIdIsNotFound(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", callBody("call.join", "call/../x")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.join", "not_found", "[MS5035]: ")
}

func TestLeavingNeedsADevice(t *testing.T) {
	fixture := newSignalFixture()
	fixture.authenticate("ca", "ana-token")
	if err := fixture.send("ca", callBody("call.leave", "call_none")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "call.leave", "invalid_request", "[MS5025]: ")
}
