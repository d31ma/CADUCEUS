package transport_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/edge/calls/desk"
	"mesail.dev/edge/calls/fakerealtime"
	"mesail.dev/edge/calls/media"
	"mesail.dev/edge/calls/transport"
	"mesail.dev/packages/oidc"
)

type staticKeys struct {
	set oidc.KeySet
}

func (keys staticKeys) Fetch(url string) (oidc.KeySet, error) {
	return keys.set, nil
}

type routeFixture struct {
	server *httptest.Server
	keys   fakerealtime.JoinKeys
	calls  *media.Calls
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newRouteFixture(t *testing.T) routeFixture {
	t.Helper()
	keys, err := fakerealtime.JoinKeys{}.Generated()
	must(t, err)
	set, err := keys.Set()
	must(t, err)
	connection, err := net.ListenPacket("udp4", "127.0.0.1:0")
	must(t, err)
	factory := media.NewPeerFactory(media.PeerOptions{IncludeLoopback: true, DisconnectedTimeout: time.Second, FailedTimeout: 2 * time.Second, KeepAliveInterval: time.Second, StartBitrate: 1_000_000}, connection)
	calls := media.NewCalls(media.NewMemberOpener(factory, admission.SystemClock{}, time.Minute), media.CallContext{Options: media.CallOptions{PlanInterval: 100 * time.Millisecond}, Clock: admission.SystemClock{}, Log: media.NewEventLog(io.Discard)})
	check := admission.NewJoinTokenCheck(admission.NewKeyBook(staticKeys{set: set}, "http://realtime/keys", admission.SystemClock{}), admission.SystemClock{})
	server := httptest.NewServer(transport.NewRoutes(desk.NewCallDesk(check, calls)).Handler())
	t.Cleanup(func() {
		server.Close()
		calls.Close()
		_ = connection.Close()
	})
	return routeFixture{server: server, keys: keys, calls: calls}
}

func (fixture routeFixture) token(t *testing.T, callId string, deviceId string, domainId string, lifetime time.Duration) string {
	t.Helper()
	now := time.Now()
	text, err := fixture.keys.Token(admission.JoinClaims{CallId: callId, DeviceId: deviceId, DomainId: domainId, Audience: "calls", ExpiresAt: now.Add(lifetime).Unix(), IssuedAt: now.Unix()})
	must(t, err)
	return text
}

func (fixture routeFixture) post(t *testing.T, path string, token string, body string) (int, transport.ApiError, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, fixture.server.URL+path, strings.NewReader(body))
	must(t, err)
	request.Header.Set("X-Mesail-Join", token)
	response, err := fixture.server.Client().Do(request)
	must(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	must(t, err)
	problem := transport.ApiError{}
	_ = json.Unmarshal(raw, &problem)
	return response.StatusCode, problem, raw
}

func offerText(t *testing.T) string {
	t.Helper()
	peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	must(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	_, err = peer.CreateDataChannel("calls", nil)
	must(t, err)
	offer, err := peer.CreateOffer(nil)
	must(t, err)
	must(t, peer.SetLocalDescription(offer))
	body, err := json.Marshal(desk.CallOffer{Sdp: offer.SDP})
	must(t, err)
	return string(body)
}

func expectProblem(t *testing.T, status int, problem transport.ApiError, wantStatus int, wantCode string, wantNumber string) {
	t.Helper()
	if status != wantStatus || problem.Code != wantCode || !strings.HasPrefix(problem.Message, "["+wantNumber+"]: ") {
		t.Fatalf("got %d %+v, want %d %s %s", status, problem, wantStatus, wantCode, wantNumber)
	}
}

func TestHealthAnswers(t *testing.T) {
	fixture := newRouteFixture(t)
	response, err := fixture.server.Client().Get(fixture.server.URL + "/api/v1/calls/health")
	must(t, err)
	defer response.Body.Close()
	health := transport.Health{}
	must(t, json.NewDecoder(response.Body).Decode(&health))
	if response.StatusCode != http.StatusOK || health != (transport.Health{Service: "calls", Status: "ok"}) {
		t.Fatalf("health %d %+v", response.StatusCode, health)
	}
}

func TestOfferWithAGoodTokenGetsAnAnswer(t *testing.T) {
	fixture := newRouteFixture(t)
	status, _, raw := fixture.post(t, "/api/v1/calls/call-1/offer", fixture.token(t, "call-1", "device-1", "example.com", time.Minute), offerText(t))
	answer := desk.CallAnswer{}
	must(t, json.Unmarshal(raw, &answer))
	if status != http.StatusOK || !strings.Contains(answer.Sdp, "a=candidate:") {
		t.Fatalf("status %d answer %q", status, answer.Sdp)
	}
	if fixture.calls.Size("call-1") != 1 {
		t.Fatal("the device was not seated")
	}
}

func TestBadTokensAreRefused(t *testing.T) {
	fixture := newRouteFixture(t)
	offer := offerText(t)
	cases := []struct {
		name   string
		token  string
		number string
	}{
		{name: "missing", token: "", number: "MS6501"},
		{name: "not a token", token: "abc", number: "MS6502"},
		{name: "expired", token: fixture.token(t, "call-1", "device-1", "example.com", -time.Second), number: "MS6503"},
		{name: "other call", token: fixture.token(t, "call-2", "device-1", "example.com", time.Minute), number: "MS6504"},
	}
	for _, testCase := range cases {
		status, problem, _ := fixture.post(t, "/api/v1/calls/call-1/offer", testCase.token, offer)
		expectProblem(t, status, problem, http.StatusUnauthorized, "unauthenticated", testCase.number)
	}
	if fixture.calls.Count() != 0 {
		t.Fatal("a refused offer opened a call")
	}
}

func TestOfferBodiesAreChecked(t *testing.T) {
	fixture := newRouteFixture(t)
	token := fixture.token(t, "call-1", "device-1", "example.com", time.Minute)
	status, problem, _ := fixture.post(t, "/api/v1/calls/call-1/offer", token, "{")
	expectProblem(t, status, problem, http.StatusBadRequest, "invalid_request", "MS6507")
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/offer", token, `{"sdp":"`+strings.Repeat("a", 300<<10)+`"}`)
	expectProblem(t, status, problem, http.StatusRequestEntityTooLarge, "too_large", "MS6508")
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/offer", token, `{"sdp":"v=0"}`)
	expectProblem(t, status, problem, http.StatusBadRequest, "invalid_request", "MS6509")
	if problem.Field != "sdp" {
		t.Fatalf("field %q", problem.Field)
	}
}

func TestAnotherDomainCannotJoinACall(t *testing.T) {
	fixture := newRouteFixture(t)
	offer := offerText(t)
	status, _, _ := fixture.post(t, "/api/v1/calls/call-1/offer", fixture.token(t, "call-1", "device-1", "example.com", time.Minute), offer)
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	status, problem, _ := fixture.post(t, "/api/v1/calls/call-1/offer", fixture.token(t, "call-1", "device-2", "example.org", time.Minute), offer)
	expectProblem(t, status, problem, http.StatusUnauthorized, "unauthenticated", "MS6505")
}

func TestCandidatesAndLeaveNeedASeat(t *testing.T) {
	fixture := newRouteFixture(t)
	candidate := `{"candidates":[{"candidate":"candidate:1 1 udp 2130706431 127.0.0.1 50000 typ host","sdpMid":"0","sdpMLineIndex":0}]}`
	stranger := fixture.token(t, "call-1", "device-9", "example.com", time.Minute)
	status, problem, _ := fixture.post(t, "/api/v1/calls/call-1/candidates", stranger, candidate)
	expectProblem(t, status, problem, http.StatusNotFound, "not_found", "MS6511")
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/leave", stranger, "")
	expectProblem(t, status, problem, http.StatusNotFound, "not_found", "MS6511")
	token := fixture.token(t, "call-1", "device-1", "example.com", time.Minute)
	status, _, _ = fixture.post(t, "/api/v1/calls/call-1/offer", token, offerText(t))
	if status != http.StatusOK {
		t.Fatalf("offer status %d", status)
	}
	status, _, _ = fixture.post(t, "/api/v1/calls/call-1/candidates", token, candidate)
	if status != http.StatusNoContent {
		t.Fatalf("candidates status %d", status)
	}
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/candidates", token, `{"candidates":[]}`)
	expectProblem(t, status, problem, http.StatusBadRequest, "invalid_request", "MS6510")
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/candidates", token, "[")
	expectProblem(t, status, problem, http.StatusBadRequest, "invalid_request", "MS6507")
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/candidates", "", candidate)
	expectProblem(t, status, problem, http.StatusUnauthorized, "unauthenticated", "MS6501")
	expired := fixture.token(t, "call-1", "device-1", "example.com", -time.Minute)
	status, _, _ = fixture.post(t, "/api/v1/calls/call-1/leave", expired, "")
	if status != http.StatusNoContent {
		t.Fatalf("leave with the joined device's old token: %d", status)
	}
	status, problem, _ = fixture.post(t, "/api/v1/calls/call-1/leave", token, "")
	expectProblem(t, status, problem, http.StatusNotFound, "not_found", "MS6511")
}

func TestRepliesCarryNoStoreAndJson(t *testing.T) {
	fixture := newRouteFixture(t)
	request, err := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/v1/calls/call-1/leave", bytes.NewReader(nil))
	must(t, err)
	response, err := fixture.server.Client().Do(request)
	must(t, err)
	defer response.Body.Close()
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers %v", response.Header)
	}
}
