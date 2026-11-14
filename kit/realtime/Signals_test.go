package realtime

import (
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func signalBody(targets string, kind string, ciphertext string) string {
	return `{"type":"signal","requestId":"r1","toDeviceIds":[` + targets + `],"kind":"` + kind + `","ciphertext":"` + ciphertext + `"}`
}

func refusedWith(t *testing.T, fixture signalFixture, connectionId string, requestType string, state string, message string) {
	t.Helper()
	frame := fixture.last(connectionId)
	if frame["type"] != "refused" || frame["requestType"] != requestType || frame["state"] != state || !strings.HasPrefix(frame["message"].(string), message) {
		t.Fatalf("got %+v", frame)
	}
}

func TestMembersExchangeOfferAnswerAndCandidates(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cb", "ben-token", "b1")
	if err := fixture.send("ca", signalBody(`"b1","b2"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	received := fixture.last("cb")
	want := map[string]any{"type": "signal", "fromDeviceId": "a1", "fromPersonId": "ana", "kind": "rtc", "ciphertext": "b2ZmZXI"}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("ben got %+v", received)
	}
	result := fixture.last("ca")
	if result["type"] != "signal.result" || result["requestId"] != "r1" || !reflect.DeepEqual(result["delivered"], []any{"b1"}) || !reflect.DeepEqual(result["offline"], []any{"b2"}) {
		t.Fatalf("ana got %+v", result)
	}
	if err := fixture.send("cb", signalBody(`"a1"`, "rtc", "YW5zd2Vy")); err != nil {
		t.Fatal(err)
	}
	if fixture.last("ca")["ciphertext"] != "YW5zd2Vy" || fixture.last("ca")["fromDeviceId"] != "b1" {
		t.Fatalf("ana got %+v", fixture.last("ca"))
	}
}

func TestASignalToADeviceOutsideTheDomainIsRefusedAndSentToNobody(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cb", "ben-token", "b1")
	if err := fixture.send("ca", signalBody(`"b1","stranger"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "not_found", "[MS5031]: ")
	if len(fixture.pushes.framesOf("cb")) != 0 {
		t.Fatalf("ben got %+v", fixture.pushes.framesOf("cb"))
	}
}

func TestASignalDoesNotCrossToAnotherDomain(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cx", "other-token", "b1")
	if err := fixture.send("ca", signalBody(`"b1"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	if len(fixture.pushes.framesOf("cx")) != 0 || !reflect.DeepEqual(fixture.last("ca")["offline"], []any{"b1"}) {
		t.Fatalf("got %+v and %+v", fixture.pushes.framesOf("cx"), fixture.last("ca"))
	}
}

func TestASignalNeedsTheSendersDevice(t *testing.T) {
	fixture := newSignalFixture()
	fixture.authenticate("ca", "ana-token")
	if err := fixture.send("ca", signalBody(`"b1"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5025]: ")
}

func TestAnUnlinkedDeviceMayNotSignalMembers(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("cn", "ana-token", "new-device")
	fixture.device("cb", "ben-token", "b1")
	if err := fixture.send("cn", signalBody(`"b1"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cn", "signal", "not_found", "[MS5031]: ")
}

func TestASignalWithAnUnknownKindIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", signalBody(`"b1"`, "chat", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5029]: must be rtc, call or linking")
}

func TestASignalWithNoTargetsIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", signalBody(``, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5028]: ")
}

func TestASignalToTooManyDevicesIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	targets := strings.TrimSuffix(strings.Repeat(`"b1",`, 201), ",")
	if err := fixture.send("ca", signalBody(targets, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5028]: ")
}

func TestASignalWithCiphertextThatIsNotBase64UrlIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", signalBody(`"b1"`, "rtc", "not base64!")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5030]: ")
}

func TestASignalWithCiphertextOverTheLimitIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	large := base64.RawURLEncoding.EncodeToString(randomBytes(65537))
	if err := fixture.send("ca", signalBody(`"b1"`, "rtc", large)); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5030]: ")
}

func TestASignalAtTheCiphertextLimitIsSent(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	largest := base64.RawURLEncoding.EncodeToString(randomBytes(65536))
	if err := fixture.send("ca", signalBody(`"b1"`, "linking", largest)); err != nil {
		t.Fatal(err)
	}
	if fixture.last("ca")["type"] != "signal.result" {
		t.Fatalf("got %+v", fixture.last("ca"))
	}
}

func TestASignalWithALongRequestIdIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	body := `{"type":"signal","requestId":"` + strings.Repeat("r", 65) + `","toDeviceIds":["b1"],"kind":"rtc","ciphertext":"b2ZmZXI"}`
	if err := fixture.send("ca", body); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "invalid_request", "[MS5043]: ")
}

func TestASignalWhileTheDirectoryIsDownIsRefusedAsUnavailable(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.directory.down = true
	if err := fixture.send("ca", signalBody(`"b1"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "signal", "unavailable", "[MS5045]: ")
}

func TestASignalFromASocketThatIsNotSignedInClosesIt(t *testing.T) {
	fixture := newSignalFixture()
	if err := fixture.send("ghost", signalBody(`"b1"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
	if fixture.pushes.closes["ghost"] != 4401 {
		t.Fatalf("got %v", fixture.pushes.closes)
	}
}

func TestARefusalToAGoneSocketIsNotAnError(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.pushes.gone["ca"] = true
	if err := fixture.send("ca", signalBody(`"stranger"`, "rtc", "b2ZmZXI")); err != nil {
		t.Fatal(err)
	}
}

func TestARefusalThatCannotBePushedIsReported(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.pushes.down = true
	if err := fixture.send("ca", signalBody(`"stranger"`, "rtc", "b2ZmZXI")); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAnUnknownFrameTypeIsStillInvalid(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	failure := Invalid{}
	if err := fixture.send("ca", `{"type":"teleport"}`); !errors.As(err, &failure) || failure.Number() != 5003 {
		t.Fatalf("got %v", err)
	}
}

func TestSignInWithADeviceOfAnotherPersonClosesTheSocket(t *testing.T) {
	fixture := newSignalFixture()
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "ca", Body: `{"type":"auth","token":"ana-token","deviceId":"b1"}`})
	if err != nil || fixture.pushes.closes["ca"] != 4401 {
		t.Fatalf("got %v, %v", err, fixture.pushes.closes)
	}
}

func TestSignInWithABadDeviceIdIsInvalid(t *testing.T) {
	fixture := newSignalFixture()
	failure := Invalid{}
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "ca", Body: `{"type":"auth","token":"ana-token","deviceId":"a/1"}`})
	if !errors.As(err, &failure) || failure.Number() != 5026 {
		t.Fatalf("got %v", err)
	}
}

func TestSignInWithoutADeviceDoesNotAskTheDirectory(t *testing.T) {
	fixture := newSignalFixture()
	fixture.authenticate("ca", "ana-token")
	if fixture.directory.asked != 0 {
		t.Fatalf("asked %d times", fixture.directory.asked)
	}
}

func TestSignInFailsWhileTheDirectoryIsDown(t *testing.T) {
	fixture := newSignalFixture()
	fixture.directory.down = true
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "ca", Body: `{"type":"auth","token":"ana-token","deviceId":"a1"}`})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}
