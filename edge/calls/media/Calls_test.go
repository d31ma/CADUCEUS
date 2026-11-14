package media

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/failures"
)

func seat(callId string, deviceId string) Seat {
	return Seat{CallId: callId, DeviceId: deviceId, DomainId: "example.com"}
}

func problemNumber(err error) int {
	return failures.Problems{}.From(err).Number()
}

func TestThreePeopleGetEachOthersFramesThatTheServerCannotRead(t *testing.T) {
	server := newTestServer(t, quickTimes())
	key := sharedKey(t)
	clients := []*testClient{}
	for index := range 3 {
		client := newTestClient(t, fmt.Sprintf("device-%d", index), key, fullShape())
		client.join(t, server, seat("call-3", client.name))
		clients = append(clients, client)
	}
	for _, client := range clients {
		client.waitConnected(t)
		client.send(false)
	}
	start := time.Now()
	for _, client := range clients {
		eventually(t, 10*time.Second, func() bool {
			return len(client.sendersSince("video", start)) == 2 && len(client.sendersSince("audio", start)) == 2
		}, client.name+" did not get video and audio from both others")
	}
	clients[0].askPicture(t)
	for _, client := range clients {
		client.lock.Lock()
		undecrypted := client.undecrypted
		client.lock.Unlock()
		if undecrypted != 0 {
			t.Fatalf("%s could not open %d forwarded frames", client.name, undecrypted)
		}
		if client.sendersSince("video", time.Time{})[client.name] {
			t.Fatalf("%s got its own video back", client.name)
		}
	}
	assertForwardedUnchanged(t, clients)
	eventually(t, 5*time.Second, func() bool {
		clients[1].lock.Lock()
		defer clients[1].lock.Unlock()
		return clients[1].pictureLoss > 0
	}, "no keyframe request reached a sender")
	if strings.Contains(server.logText(), "device-0:video") {
		t.Fatal("the server logged frame content")
	}
}

func assertForwardedUnchanged(t *testing.T, clients []*testClient) {
	t.Helper()
	sent := map[string]bool{}
	for _, client := range clients {
		client.lock.Lock()
		for payload := range client.sentPayloads {
			sent[payload] = true
		}
		client.lock.Unlock()
	}
	for _, client := range clients {
		client.lock.Lock()
		for payload := range client.gotPayloads {
			if !sent[payload] {
				client.lock.Unlock()
				t.Fatalf("%s got a payload that no device sent", client.name)
			}
		}
		client.lock.Unlock()
	}
}

func TestTwelvePeopleSeeAtMostNineVideosWithTheSpeakerFirst(t *testing.T) {
	server := newTestServer(t, quickTimes())
	key := sharedKey(t)
	clients := []*testClient{}
	for index := range 12 {
		client := newTestClient(t, fmt.Sprintf("device-%02d", index), key, fullShape())
		client.join(t, server, seat("call-12", client.name))
		clients = append(clients, client)
	}
	speaker := clients[11]
	for _, client := range clients {
		client.waitConnected(t)
		client.send(client == speaker)
	}
	time.Sleep(2 * time.Second)
	since := time.Now()
	time.Sleep(1500 * time.Millisecond)
	for _, client := range clients[:11] {
		videos := client.sendersSince("video", since)
		if len(videos) > 9 {
			t.Fatalf("%s got %d videos", client.name, len(videos))
		}
		if !videos[speaker.name] {
			t.Fatalf("%s does not get the speaker's video: %v", client.name, videos)
		}
		message := client.lastMessage()
		if len(message.Order) != 11 || message.Order[0] != speaker.name {
			t.Fatalf("%s order %v", client.name, message.Order)
		}
		shown := slices.DeleteFunc(slices.Clone(message.Video), func(deviceId string) bool { return deviceId == "" })
		if len(shown) != 9 || !slices.Contains(shown, speaker.name) {
			t.Fatalf("%s video slots %v", client.name, message.Video)
		}
		if !client.sendersSince("audio", since)[speaker.name] {
			t.Fatalf("%s does not hear the speaker", client.name)
		}
	}
}

func TestTheSharedScreenGoesToItsOwnSlot(t *testing.T) {
	server := newTestServer(t, quickTimes())
	key := sharedKey(t)
	sharer := newTestClient(t, "sharer", key, clientShape{videoSlots: 1, audioSlots: 1, screenSlot: true, sendScreen: true})
	viewer := newTestClient(t, "viewer", key, clientShape{videoSlots: 1, audioSlots: 1, screenSlot: true})
	sharer.join(t, server, seat("call-screen", "sharer"))
	viewer.join(t, server, seat("call-screen", "viewer"))
	sharer.waitConnected(t)
	viewer.waitConnected(t)
	sharer.send(false)
	viewer.send(false)
	eventually(t, 10*time.Second, func() bool {
		return viewer.sendersSince("screen", time.Time{})["sharer"] && viewer.lastMessage().Screen == "sharer"
	}, "the viewer did not get the shared screen")
}

func TestLeaveAndLostConnectionsCleanUp(t *testing.T) {
	server := newTestServer(t, quickTimes())
	key := sharedKey(t)
	first := newTestClient(t, "first", key, fullShape())
	second := newTestClient(t, "second", key, fullShape())
	first.join(t, server, seat("call-clean", "first"))
	second.join(t, server, seat("call-clean", "second"))
	first.waitConnected(t)
	second.waitConnected(t)
	if server.calls.Size("call-clean") != 2 {
		t.Fatalf("size %d", server.calls.Size("call-clean"))
	}
	second.close()
	eventually(t, 10*time.Second, func() bool { return server.calls.Size("call-clean") == 1 }, "the lost device was not removed")
	must(t, server.calls.Leave(seat("call-clean", "first")))
	eventually(t, 5*time.Second, func() bool { return server.calls.Count() == 0 }, "the empty call was not ended")
	if problemNumber(server.calls.Leave(seat("call-clean", "first"))) != 6511 {
		t.Fatal("a second leave was not refused")
	}
	logs := server.logText()
	if !strings.Contains(logs, "connection lost") || !strings.Contains(logs, "call.ended") {
		t.Fatalf("logs %s", logs)
	}
}

func TestADeviceThatNeverConnectsIsRemoved(t *testing.T) {
	times := quickTimes()
	times.connectWait = 500 * time.Millisecond
	server := newTestServer(t, times)
	_, err := server.calls.Join(seat("call-silent", "silent"), offerFrom(t, newChannelOnlyClient(t)))
	must(t, err)
	eventually(t, 5*time.Second, func() bool { return server.calls.Count() == 0 }, "the silent device was not removed")
}

func offerFrom(t *testing.T, peer *webrtc.PeerConnection) string {
	t.Helper()
	offer, err := peer.CreateOffer(nil)
	must(t, err)
	gathered := webrtc.GatheringCompletePromise(peer)
	must(t, peer.SetLocalDescription(offer))
	<-gathered
	return peer.LocalDescription().SDP
}

func TestTheFiftyFirstDeviceIsRefused(t *testing.T) {
	times := quickTimes()
	times.connectWait = time.Minute
	server := newTestServer(t, times)
	offer := offerFrom(t, newChannelOnlyClient(t))
	for index := range 50 {
		_, err := server.calls.Join(seat("call-full", fmt.Sprintf("device-%d", index)), offer)
		must(t, err)
	}
	_, err := server.calls.Join(seat("call-full", "device-50"), offer)
	if problemNumber(err) != 6506 {
		t.Fatalf("error %v", err)
	}
	_, err = server.calls.Join(seat("call-full", "device-7"), offer)
	if err != nil {
		t.Fatalf("a device that joins again was refused: %v", err)
	}
	if server.calls.Size("call-full") != 50 {
		t.Fatalf("size %d", server.calls.Size("call-full"))
	}
}

func TestJoinRefusesBadOffersAndOtherDomains(t *testing.T) {
	server := newTestServer(t, quickTimes())
	_, err := server.calls.Join(seat("call-bad", "device-1"), "not an offer")
	if problemNumber(err) != 6509 {
		t.Fatalf("error %v", err)
	}
	client := newTestClient(t, "many", sharedKey(t), clientShape{videoSlots: 9, audioSlots: 50})
	_, err = server.calls.Join(seat("call-bad", "many"), client.offer(t))
	if problemNumber(err) != 6509 {
		t.Fatalf("error %v", err)
	}
	if server.calls.Count() != 0 {
		t.Fatal("a refused join left a call behind")
	}
	offer := offerFrom(t, newChannelOnlyClient(t))
	_, err = server.calls.Join(seat("call-domain", "device-1"), offer)
	must(t, err)
	_, err = server.calls.Join(Seat{CallId: "call-domain", DeviceId: "device-2", DomainId: "example.org"}, offer)
	if problemNumber(err) != 6505 {
		t.Fatalf("error %v", err)
	}
	if problemNumber(server.calls.Leave(Seat{CallId: "call-domain", DeviceId: "device-1", DomainId: "example.org"})) != 6511 {
		t.Fatal("a leave from another domain was not refused")
	}
}

func TestCandidatesAreAddedOnlyForSeatedDevices(t *testing.T) {
	server := newTestServer(t, quickTimes())
	_, err := server.calls.Join(seat("call-ice", "device-1"), offerFrom(t, newChannelOnlyClient(t)))
	must(t, err)
	good := []webrtc.ICECandidateInit{{Candidate: "candidate:1 1 udp 2130706431 127.0.0.1 50000 typ host"}}
	must(t, server.calls.AddCandidates(seat("call-ice", "device-1"), good))
	bad := []webrtc.ICECandidateInit{{Candidate: "nonsense"}}
	if problemNumber(server.calls.AddCandidates(seat("call-ice", "device-1"), bad)) != 6510 {
		t.Fatal("a bad candidate was accepted")
	}
	if problemNumber(server.calls.AddCandidates(seat("call-ice", "device-9"), good)) != 6511 {
		t.Fatal("candidates for an unknown device were accepted")
	}
	if problemNumber(server.calls.AddCandidates(seat("call-none", "device-1"), good)) != 6511 {
		t.Fatal("candidates for an unknown call were accepted")
	}
}

func TestTheAnswerUsesOnlyTheSharedPort(t *testing.T) {
	server := newTestServer(t, quickTimes())
	answer, err := server.calls.Join(seat("call-port", "device-1"), offerFrom(t, newChannelOnlyClient(t)))
	must(t, err)
	port := fmt.Sprint(server.connection.LocalAddr().(interface{ String() string }).String())
	port = port[strings.LastIndex(port, ":")+1:]
	candidates := 0
	for _, line := range strings.Split(answer, "\n") {
		if strings.HasPrefix(line, "a=candidate:") {
			candidates++
			fields := strings.Fields(line)
			if fields[5] != port {
				t.Fatalf("candidate %q does not use port %s", line, port)
			}
		}
	}
	if candidates == 0 {
		t.Fatal("the answer has no candidates")
	}
}
