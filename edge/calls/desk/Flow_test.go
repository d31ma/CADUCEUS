package desk

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/edge/calls/failures"
	"mesail.dev/edge/calls/fakerealtime"
	"mesail.dev/edge/calls/media"
	"mesail.dev/packages/oidc"
)

type staticKeys struct {
	set oidc.KeySet
}

func (keys staticKeys) Fetch(url string) (oidc.KeySet, error) {
	return keys.set, nil
}

type flow struct {
	desk  CallDesk
	keys  fakerealtime.JoinKeys
	calls *media.Calls
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFlow(t *testing.T) flow {
	t.Helper()
	keys, err := fakerealtime.JoinKeys{}.Generated()
	must(t, err)
	set, err := keys.Set()
	must(t, err)
	connection, err := net.ListenPacket("udp4", "127.0.0.1:0")
	must(t, err)
	factory := media.NewPeerFactory(media.PeerOptions{IncludeLoopback: true, DisconnectedTimeout: time.Second, FailedTimeout: 2 * time.Second, KeepAliveInterval: time.Second, StartBitrate: 1_000_000}, connection)
	calls := media.NewCalls(media.NewMemberOpener(factory, admission.SystemClock{}, time.Minute), media.CallContext{Options: media.CallOptions{PlanInterval: 100 * time.Millisecond}, Clock: admission.SystemClock{}, Log: media.NewEventLog(io.Discard)})
	t.Cleanup(func() {
		calls.Close()
		_ = connection.Close()
	})
	check := admission.NewJoinTokenCheck(admission.NewKeyBook(staticKeys{set: set}, "http://realtime/keys", admission.SystemClock{}), admission.SystemClock{})
	return flow{desk: NewCallDesk(check, calls), keys: keys, calls: calls}
}

func (flow flow) token(t *testing.T, lifetime time.Duration) string {
	t.Helper()
	now := time.Now()
	text, err := flow.keys.Token(admission.JoinClaims{CallId: "call-1", DeviceId: "device-1", DomainId: "example.com", Audience: "calls", ExpiresAt: now.Add(lifetime).Unix(), IssuedAt: now.Unix()})
	must(t, err)
	return text
}

func offerSdp(t *testing.T) string {
	t.Helper()
	peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	must(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	_, err = peer.CreateDataChannel("calls", nil)
	must(t, err)
	offer, err := peer.CreateOffer(nil)
	must(t, err)
	must(t, peer.SetLocalDescription(offer))
	return offer.SDP
}

func decoding(value any, err error) func(into any) error {
	return func(into any) error {
		switch target := into.(type) {
		case *CallOffer:
			*target = value.(CallOffer)
		case *CallCandidates:
			*target = value.(CallCandidates)
		}
		return err
	}
}

func number(err error) int {
	return failures.Problems{}.From(err).Number()
}

func TestOfferChecksTheTokenThenTheBodyThenJoins(t *testing.T) {
	flow := newFlow(t)
	_, err := flow.desk.Offer(Request{CallId: "call-1", Decode: decoding(CallOffer{}, nil)})
	if number(err) != 6501 {
		t.Fatalf("error %v", err)
	}
	broken := failures.Problems{}.InvalidJson()
	_, err = flow.desk.Offer(Request{Token: flow.token(t, time.Minute), CallId: "call-1", Decode: decoding(CallOffer{}, broken)})
	if !errors.Is(err, broken) {
		t.Fatalf("error %v", err)
	}
	answer, err := flow.desk.Offer(Request{Token: flow.token(t, time.Minute), CallId: "call-1", Decode: decoding(CallOffer{Sdp: offerSdp(t)}, nil)})
	if err != nil || answer.Sdp == "" || flow.calls.Size("call-1") != 1 {
		t.Fatalf("answer %q error %v", answer.Sdp, err)
	}
}

func TestCandidatesAndLeaveFollowTheSeat(t *testing.T) {
	flow := newFlow(t)
	candidates := CallCandidates{Candidates: []webrtc.ICECandidateInit{{Candidate: "candidate:1 1 udp 2130706431 127.0.0.1 50000 typ host"}}}
	must(t, errorOf(flow.desk.Offer(Request{Token: flow.token(t, time.Minute), CallId: "call-1", Decode: decoding(CallOffer{Sdp: offerSdp(t)}, nil)})))
	expired := flow.token(t, -time.Minute)
	if number(flow.desk.AddCandidates(Request{CallId: "call-1", Decode: decoding(candidates, nil)})) != 6501 {
		t.Fatal("candidates without a token were accepted")
	}
	broken := failures.Problems{}.TooLarge()
	if !errors.Is(flow.desk.AddCandidates(Request{Token: expired, CallId: "call-1", Decode: decoding(candidates, broken)}), broken) {
		t.Fatal("a body error was lost")
	}
	if number(flow.desk.AddCandidates(Request{Token: expired, CallId: "call-1", Decode: decoding(CallCandidates{}, nil)})) != 6510 {
		t.Fatal("an empty list was accepted")
	}
	must(t, flow.desk.AddCandidates(Request{Token: expired, CallId: "call-1", Decode: decoding(candidates, nil)}))
	if number(flow.desk.Leave(Request{CallId: "call-1"})) != 6501 {
		t.Fatal("leave without a token was accepted")
	}
	must(t, flow.desk.Leave(Request{Token: expired, CallId: "call-1"}))
	if number(flow.desk.Leave(Request{Token: expired, CallId: "call-1"})) != 6511 {
		t.Fatal("a second leave was accepted")
	}
}

func errorOf(answer CallAnswer, err error) error {
	return err
}
