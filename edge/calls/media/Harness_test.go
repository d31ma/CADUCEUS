package media

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/admission"
)

type testServer struct {
	calls      *Calls
	connection net.PacketConn
	logs       *lockedWriter
}

func (server *testServer) logText() string {
	server.logs.lock.Lock()
	defer server.logs.lock.Unlock()
	return server.logs.target.(*bytes.Buffer).String()
}

type serverTimes struct {
	connectWait  time.Duration
	failed       time.Duration
	planInterval time.Duration
}

func quickTimes() serverTimes {
	return serverTimes{connectWait: 10 * time.Second, failed: 2 * time.Second, planInterval: 100 * time.Millisecond}
}

func newTestServer(t *testing.T, times serverTimes) *testServer {
	t.Helper()
	connection, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	factory := NewPeerFactory(PeerOptions{
		IncludeLoopback:     true,
		DisconnectedTimeout: time.Second,
		FailedTimeout:       times.failed,
		KeepAliveInterval:   200 * time.Millisecond,
		StartBitrate:        5_000_000,
	}, connection)
	logs := &lockedWriter{target: &bytes.Buffer{}}
	calls := NewCalls(NewMemberOpener(factory, admission.SystemClock{}, times.connectWait), CallContext{
		Options: CallOptions{PlanInterval: times.planInterval},
		Clock:   admission.SystemClock{},
		Log:     NewEventLog(logs),
	})
	server := &testServer{calls: calls, connection: connection, logs: logs}
	t.Cleanup(func() {
		calls.Close()
		_ = connection.Close()
	})
	return server
}

type lockedWriter struct {
	lock   sync.Mutex
	target io.Writer
}

func (writer *lockedWriter) Write(data []byte) (int, error) {
	writer.lock.Lock()
	defer writer.lock.Unlock()
	return writer.target.Write(data)
}

type clientShape struct {
	videoSlots int
	audioSlots int
	screenSlot bool
	sendScreen bool
}

func fullShape() clientShape {
	return clientShape{videoSlots: 9, audioSlots: 3, screenSlot: true}
}

type received struct {
	slot   int
	sender string
	kind   string
	at     time.Time
}

type testClient struct {
	name         string
	peer         *webrtc.PeerConnection
	audio        *webrtc.TrackLocalStaticRTP
	video        *webrtc.TrackLocalStaticRTP
	screen       *webrtc.TrackLocalStaticRTP
	key          []byte
	connected    chan struct{}
	lock         sync.Mutex
	got          []received
	sentPayloads map[string]bool
	gotPayloads  map[string]bool
	undecrypted  int
	messages     []SlotMessage
	pictureLoss  int
	slotTracks   map[int]*webrtc.TrackRemote
	stop         chan struct{}
	stopOnce     sync.Once
}

func clientApi(t *testing.T) *webrtc.API {
	t.Helper()
	engine := &webrtc.MediaEngine{}
	must(t, engine.RegisterCodec(Codecs{}.Opus(), webrtc.RTPCodecTypeAudio))
	must(t, engine.RegisterCodec(Codecs{}.Vp8(), webrtc.RTPCodecTypeVideo))
	must(t, engine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: sdp.AudioLevelURI}, webrtc.RTPCodecTypeAudio))
	registry := &interceptor.Registry{}
	must(t, webrtc.RegisterDefaultInterceptors(engine, registry))
	settings := webrtc.SettingEngine{}
	settings.SetIncludeLoopbackCandidate(true)
	settings.SetInterfaceFilter(func(name string) bool { return name == "lo0" || name == "lo" })
	settings.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	return webrtc.NewAPI(webrtc.WithMediaEngine(engine), webrtc.WithSettingEngine(settings), webrtc.WithInterceptorRegistry(registry))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newTestClient(t *testing.T, name string, key []byte, shape clientShape) *testClient {
	t.Helper()
	peer, err := clientApi(t).NewPeerConnection(webrtc.Configuration{})
	must(t, err)
	client := &testClient{name: name, peer: peer, key: key, connected: make(chan struct{}), sentPayloads: map[string]bool{}, gotPayloads: map[string]bool{}, slotTracks: map[int]*webrtc.TrackRemote{}, stop: make(chan struct{})}
	client.audio, err = webrtc.NewTrackLocalStaticRTP(Codecs{}.Opus().RTPCodecCapability, "audio", name)
	must(t, err)
	client.video, err = webrtc.NewTrackLocalStaticRTP(Codecs{}.Vp8().RTPCodecCapability, "video", name)
	must(t, err)
	client.addSending(t, client.audio)
	client.addSending(t, client.video)
	if shape.sendScreen {
		client.screen, err = webrtc.NewTrackLocalStaticRTP(Codecs{}.Vp8().RTPCodecCapability, "screen", name)
		must(t, err)
		client.addSending(t, client.screen)
	}
	if shape.screenSlot {
		client.addReceiving(t, webrtc.RTPCodecTypeVideo, 1)
	}
	client.addReceiving(t, webrtc.RTPCodecTypeVideo, shape.videoSlots)
	client.addReceiving(t, webrtc.RTPCodecTypeAudio, shape.audioSlots)
	channel, err := peer.CreateDataChannel("calls", nil)
	must(t, err)
	channel.OnMessage(client.keepMessage)
	peer.OnTrack(client.trackArrived)
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(client.connected)
		}
	})
	t.Cleanup(client.close)
	return client
}

func newChannelOnlyClient(t *testing.T) *webrtc.PeerConnection {
	t.Helper()
	peer, err := clientApi(t).NewPeerConnection(webrtc.Configuration{})
	must(t, err)
	_, err = peer.CreateDataChannel("calls", nil)
	must(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	return peer
}

func (client *testClient) addSending(t *testing.T, track *webrtc.TrackLocalStaticRTP) {
	transceiver, err := client.peer.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})
	must(t, err)
	go client.readFeedback(transceiver.Sender())
}

func (client *testClient) addReceiving(t *testing.T, kind webrtc.RTPCodecType, count int) {
	for range count {
		_, err := client.peer.AddTransceiverFromKind(kind, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
		must(t, err)
	}
}

func (client *testClient) readFeedback(sender *webrtc.RTPSender) {
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			return
		}
		client.countPictureLoss(packets)
	}
}

func (client *testClient) countPictureLoss(packets []rtcp.Packet) {
	client.lock.Lock()
	defer client.lock.Unlock()
	for _, packet := range packets {
		if _, ok := packet.(*rtcp.PictureLossIndication); ok {
			client.pictureLoss++
		}
	}
}

func (client *testClient) keepMessage(message webrtc.DataChannelMessage) {
	slots := SlotMessage{}
	if json.Unmarshal(message.Data, &slots) != nil {
		return
	}
	client.lock.Lock()
	defer client.lock.Unlock()
	client.messages = append(client.messages, slots)
}

func (client *testClient) trackArrived(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	slot := -1
	for index, transceiver := range client.peer.GetTransceivers() {
		if transceiver.Receiver() == receiver {
			slot = index
		}
	}
	client.lock.Lock()
	client.slotTracks[slot] = track
	client.lock.Unlock()
	go client.readTrack(track, slot)
}

func (client *testClient) readTrack(track *webrtc.TrackRemote, slot int) {
	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		client.keepPacket(slot, packet)
	}
}

func (client *testClient) keepPacket(slot int, packet *rtp.Packet) {
	plain, err := client.open(packet.Payload)
	client.lock.Lock()
	defer client.lock.Unlock()
	if err != nil {
		client.undecrypted++
		return
	}
	client.gotPayloads[string(packet.Payload)] = true
	parts := strings.Split(string(plain), ":")
	client.got = append(client.got, received{slot: slot, sender: parts[0], kind: parts[1], at: time.Now()})
}

func (client *testClient) seal(plain []byte) []byte {
	block, _ := aes.NewCipher(client.key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	_, _ = rand.Read(nonce)
	return aead.Seal(nonce, nonce, plain, nil)
}

func (client *testClient) open(sealed []byte) ([]byte, error) {
	block, _ := aes.NewCipher(client.key)
	aead, _ := cipher.NewGCM(block)
	if len(sealed) < aead.NonceSize() {
		return nil, fmt.Errorf("short payload")
	}
	return aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], nil)
}

func (client *testClient) offer(t *testing.T) string {
	t.Helper()
	offer, err := client.peer.CreateOffer(nil)
	must(t, err)
	gathered := webrtc.GatheringCompletePromise(client.peer)
	must(t, client.peer.SetLocalDescription(offer))
	<-gathered
	return client.peer.LocalDescription().SDP
}

func (client *testClient) accept(t *testing.T, answer string) {
	t.Helper()
	must(t, client.peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}))
}

func (client *testClient) join(t *testing.T, server *testServer, seat Seat) {
	t.Helper()
	answer, err := server.calls.Join(seat, client.offer(t))
	must(t, err)
	client.accept(t, answer)
}

func (client *testClient) waitConnected(t *testing.T) {
	t.Helper()
	select {
	case <-client.connected:
	case <-time.After(15 * time.Second):
		t.Fatalf("%s did not connect", client.name)
	}
}

func (client *testClient) send(loud bool) {
	go client.sendLoop(loud)
}

func (client *testClient) sendLoop(loud bool) {
	level := uint8(127)
	if loud {
		level = 10
	}
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for number := 0; ; number++ {
		select {
		case <-client.stop:
			return
		case <-ticker.C:
		}
		client.write(client.audio, "audio", number, level)
		client.write(client.video, "video", number, 127)
		if client.screen != nil {
			client.write(client.screen, "screen", number, 127)
		}
	}
}

func (client *testClient) write(track *webrtc.TrackLocalStaticRTP, kind string, number int, level uint8) {
	payload := client.seal(fmt.Appendf(nil, "%s:%s:%d", client.name, kind, number))
	client.lock.Lock()
	client.sentPayloads[string(payload)] = true
	client.lock.Unlock()
	packet := &rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: uint16(number), Timestamp: uint32(number * 3000), Marker: true}, Payload: payload}
	client.addAudioLevel(packet, kind, level)
	_ = track.WriteRTP(packet)
}

func (client *testClient) addAudioLevel(packet *rtp.Packet, kind string, level uint8) {
	if kind != "audio" {
		return
	}
	for _, transceiver := range client.peer.GetTransceivers() {
		if transceiver.Sender() != nil && transceiver.Sender().Track() == client.audio {
			id := ExtensionIds{}.Find(transceiver.Sender().GetParameters().HeaderExtensions, sdp.AudioLevelURI)
			raw, _ := rtp.AudioLevelExtension{Level: level}.Marshal()
			_ = packet.SetExtension(id, raw)
		}
	}
}

func (client *testClient) askPicture(t *testing.T) {
	t.Helper()
	client.lock.Lock()
	tracks := []*webrtc.TrackRemote{}
	for _, track := range client.slotTracks {
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			tracks = append(tracks, track)
		}
	}
	client.lock.Unlock()
	for _, track := range tracks {
		must(t, client.peer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}}))
	}
}

func (client *testClient) sendersSince(kind string, since time.Time) map[string]bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	senders := map[string]bool{}
	for _, packet := range client.got {
		if packet.kind == kind && packet.at.After(since) {
			senders[packet.sender] = true
		}
	}
	return senders
}

func (client *testClient) lastMessage() SlotMessage {
	client.lock.Lock()
	defer client.lock.Unlock()
	if len(client.messages) == 0 {
		return SlotMessage{}
	}
	return client.messages[len(client.messages)-1]
}

func (client *testClient) close() {
	client.stopOnce.Do(func() { close(client.stop) })
	_ = client.peer.Close()
}

func eventually(t *testing.T, wait time.Duration, condition func() bool, failure string) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal(failure)
}

func sharedKey(t *testing.T) []byte {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	must(t, err)
	return key
}
