package media

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func people(count int, base time.Time) []SourceSnapshot {
	sources := []SourceSnapshot{}
	for index := range count {
		sources = append(sources, SourceSnapshot{DeviceId: fmt.Sprintf("device-%02d", index), JoinedAt: base.Add(time.Duration(index) * time.Second), HasMicrophone: true, CameraRids: []string{"f", "h", "q"}})
	}
	return sources
}

func emptySlots(count int) []string {
	return make([]string, count)
}

func TestRankingPutsTheLatestSpeakerFirstAndLeavesOutTheViewer(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	sources := people(4, base)
	sources[3].LastSpokeAt = base.Add(time.Minute)
	sources[1].LastSpokeAt = base.Add(30 * time.Second)
	order := SpeakerRanking{}.Order(sources, "device-00")
	ids := ReceiverPlanner{}.ids(order)
	if !slices.Equal(ids, []string{"device-03", "device-01", "device-02"}) {
		t.Fatalf("order %v", ids)
	}
}

func TestPlannerShowsNineVideosWithTheSpeakerFirstInACallOfTwelve(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	sources := people(12, base)
	sources[11].LastSpokeAt = base.Add(time.Hour)
	plan := ReceiverPlanner{}.Plan(ViewerState{DeviceId: "device-00", VideoShown: emptySlots(9), AudioShown: emptySlots(3), ScreenSlot: true, Bitrate: 3_000_000}, sources)
	if plan.Message.Order[0] != "device-11" || len(plan.Message.Order) != 11 {
		t.Fatalf("order %v", plan.Message.Order)
	}
	if plan.Video[0].DeviceId != "device-11" || plan.Video[0].Rid != "f" {
		t.Fatalf("first video %+v", plan.Video[0])
	}
	for _, feed := range plan.Video[1:] {
		if feed.Rid != "q" {
			t.Fatalf("other videos should be small: %+v", plan.Video)
		}
	}
	if slices.Contains(plan.Message.Video, "device-00") || slices.Contains(plan.Message.Video, "") {
		t.Fatalf("video slots %v", plan.Message.Video)
	}
	if !plan.Screen.Empty() || plan.Message.Screen != "" {
		t.Fatalf("screen %+v", plan.Screen)
	}
	if plan.Audio[0].DeviceId != "device-11" || plan.Audio[0].Kind != (FeedKinds{}).Microphone() {
		t.Fatalf("audio %+v", plan.Audio)
	}
}

func TestPlannerKeepsSlotsStableAndPlacesANewSpeaker(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	sources := people(11, base)
	first := ReceiverPlanner{}.Plan(ViewerState{DeviceId: "device-00", VideoShown: emptySlots(9), AudioShown: emptySlots(2), Bitrate: 0}, sources)
	if slices.Contains(first.Message.Video, "device-10") {
		t.Fatalf("the last person should wait: %v", first.Message.Video)
	}
	sources[10].LastSpokeAt = base.Add(time.Hour)
	second := ReceiverPlanner{}.Plan(ViewerState{DeviceId: "device-00", VideoShown: first.Message.Video, AudioShown: first.Message.Audio, Bitrate: 0}, sources)
	moved := 0
	for index := range second.Message.Video {
		if second.Message.Video[index] != first.Message.Video[index] {
			moved++
			if second.Message.Video[index] != "device-10" {
				t.Fatalf("slot %d got %s", index, second.Message.Video[index])
			}
		}
	}
	if moved != 1 {
		t.Fatalf("%d slots changed", moved)
	}
}

func TestPlannerLeavesOutPeopleWithoutCameraOrMicrophone(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	sources := people(3, base)
	sources[1].CameraRids = nil
	sources[2].HasMicrophone = false
	plan := ReceiverPlanner{}.Plan(ViewerState{DeviceId: "device-00", VideoShown: emptySlots(2), AudioShown: emptySlots(2), Bitrate: 1_000_000}, sources)
	if !slices.Equal(plan.Message.Video, []string{"device-02", ""}) || !plan.Video[1].Empty() {
		t.Fatalf("video %v", plan.Message.Video)
	}
	if !slices.Equal(plan.Message.Audio, []string{"device-01", ""}) || !plan.Audio[1].Empty() {
		t.Fatalf("audio %v", plan.Message.Audio)
	}
}

func TestPlannerSendsTheEarliestSharedScreen(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	sources := people(4, base)
	sources[2].ScreenSince = base.Add(time.Minute)
	sources[3].ScreenSince = base.Add(2 * time.Minute)
	plan := ReceiverPlanner{}.Plan(ViewerState{DeviceId: "device-00", ScreenSlot: true}, sources)
	if plan.Screen.DeviceId != "device-02" || plan.Screen.Kind != (FeedKinds{}).Screen() {
		t.Fatalf("screen %+v", plan.Screen)
	}
	alone := ReceiverPlanner{}.Plan(ViewerState{DeviceId: "device-02", ScreenSlot: true}, sources)
	if alone.Screen.DeviceId != "device-03" {
		t.Fatalf("the sharer should see the other screen: %+v", alone.Screen)
	}
}

func TestLayerPlanGivesTheSpeakerTheBestLayerFirst(t *testing.T) {
	cases := []struct {
		bitrate int
		count   int
		want    []int
	}{
		{bitrate: 0, count: 3, want: []int{0, 0, 0}},
		{bitrate: 900_000, count: 3, want: []int{1, 0, 0}},
		{bitrate: 2_000_000, count: 3, want: []int{2, 0, 0}},
		{bitrate: 3_000_000, count: 3, want: []int{2, 1, 1}},
		{bitrate: 10_000_000, count: 2, want: []int{2, 2}},
	}
	for _, testCase := range cases {
		got := LayerPlan{}.Choose(testCase.bitrate, testCase.count)
		if !slices.Equal(got, testCase.want) {
			t.Fatalf("bitrate %d: %v, want %v", testCase.bitrate, got, testCase.want)
		}
	}
}

func TestRidChoiceFallsBackToWhatTheSenderHas(t *testing.T) {
	cases := []struct {
		quality   int
		available []string
		want      string
	}{
		{quality: 2, available: []string{"f", "h", "q"}, want: "f"},
		{quality: 2, available: []string{"h", "q"}, want: "h"},
		{quality: 0, available: []string{"f", "h"}, want: "f"},
		{quality: 1, available: []string{""}, want: ""},
		{quality: 1, available: nil, want: ""},
	}
	for _, testCase := range cases {
		got := RidChoice{}.For(testCase.quality, testCase.available)
		if got != testCase.want {
			t.Fatalf("quality %d of %v: %q, want %q", testCase.quality, testCase.available, got, testCase.want)
		}
	}
}

func TestContinuityKeepsNumbersRunningAcrossASwitch(t *testing.T) {
	continuity := NewContinuity(3000)
	first := rtp.Header{SequenceNumber: 500, Timestamp: 90_000}
	continuity.Rewrite(&first)
	second := rtp.Header{SequenceNumber: 501, Timestamp: 93_000}
	continuity.Rewrite(&second)
	continuity.Restart()
	other := rtp.Header{SequenceNumber: 7, Timestamp: 12}
	continuity.Rewrite(&other)
	if second.SequenceNumber != first.SequenceNumber+1 || other.SequenceNumber != second.SequenceNumber+1 {
		t.Fatalf("sequence %d %d %d", first.SequenceNumber, second.SequenceNumber, other.SequenceNumber)
	}
	if other.Timestamp != second.Timestamp+3000 {
		t.Fatalf("timestamp %d after %d", other.Timestamp, second.Timestamp)
	}
}

func newSlot(t *testing.T, kind SlotKind) *SlotOutput {
	track, err := webrtc.NewTrackLocalStaticRTP(kind.Codec, "slot", "test")
	must(t, err)
	return NewSlotOutput(track, kind)
}

func TestSlotSwitchesVideoOnlyAtAKeyframeOrAfterTheWait(t *testing.T) {
	slot := newSlot(t, Codecs{}.VideoSlot())
	now := time.Unix(1_800_000_000, 0)
	first := FeedKey{DeviceId: "a", Kind: FeedKinds{}.Camera(), Rid: "q"}
	second := FeedKey{DeviceId: "a", Kind: FeedKinds{}.Camera(), Rid: "f"}
	packet := &rtp.Packet{Header: rtp.Header{Version: 2}, Payload: []byte{1}}
	if !slot.Target(first, now) || slot.Current() != first {
		t.Fatal("an empty slot should take the first feed at once")
	}
	if !slot.Target(second, now) || slot.Current() != first || slot.Shown() != "a" {
		t.Fatal("a switch should wait")
	}
	if slot.Target(second, now) {
		t.Fatal("the same target twice should not ask again")
	}
	must(t, slot.Forward(second, packet, FrameStart{New: true}, now.Add(100*time.Millisecond)))
	if slot.Current() != first {
		t.Fatal("switched before the wait")
	}
	must(t, slot.Forward(second, packet, FrameStart{Independent: true}, now.Add(150*time.Millisecond)))
	if slot.Current() != second {
		t.Fatal("did not switch at a keyframe")
	}
	slot.Target(first, now)
	must(t, slot.Forward(first, packet, FrameStart{New: true}, now.Add(time.Second)))
	if slot.Current() != first {
		t.Fatal("did not switch after the wait")
	}
	slot.Target(second, now)
	if slot.Target(first, now) || slot.Feeds()[1] != (FeedKey{}) {
		t.Fatal("going back to the current feed should cancel the switch")
	}
	if slot.Target(FeedKey{}, now) || slot.Current() != (FeedKey{}) || slot.Shown() != "" {
		t.Fatal("an empty target should clear the slot")
	}
}

func TestSlotForwardsOnlyTheCurrentFeed(t *testing.T) {
	slot := newSlot(t, Codecs{}.AudioSlot())
	now := time.Now()
	feed := FeedKey{DeviceId: "a", Kind: FeedKinds{}.Microphone()}
	slot.Target(feed, now)
	packet := &rtp.Packet{Header: rtp.Header{Version: 2, Extension: true, Extensions: []rtp.Extension{{}}}, Payload: []byte{9}}
	must(t, slot.Forward(FeedKey{DeviceId: "b"}, packet, FrameStart{New: true}, now))
	must(t, slot.Forward(feed, packet, FrameStart{New: true}, now))
	if len(packet.Header.Extensions) != 1 {
		t.Fatal("the source packet was changed")
	}
}

func TestOfferLayoutReadsSendingAndReceivingParts(t *testing.T) {
	audio, video := webrtc.RTPCodecTypeAudio, webrtc.RTPCodecTypeVideo
	sending, receiving := webrtc.RTPTransceiverDirectionRecvonly, webrtc.RTPTransceiverDirectionSendonly
	shapes := []TransceiverShape{{audio, sending}, {video, sending}, {video, sending}}
	for range 10 {
		shapes = append(shapes, TransceiverShape{video, receiving})
	}
	shapes = append(shapes, TransceiverShape{audio, receiving}, TransceiverShape{audio, webrtc.RTPTransceiverDirectionInactive})
	layout, err := OfferLayoutReader{}.Read(shapes)
	must(t, err)
	if len(layout.VideoSlots) != 9 || len(layout.ScreenSlot) != 1 || layout.ScreenSlot[0] != 3 || layout.VideoSlots[0] != 4 || len(layout.AudioSlots) != 1 {
		t.Fatalf("layout %+v", layout)
	}
	kinds := []FeedKind{layout.KindAt(0), layout.KindAt(1), layout.KindAt(2), layout.KindAt(3)}
	if !slices.Equal(kinds, []FeedKind{"microphone", "camera", "screen", ""}) {
		t.Fatalf("kinds %v", kinds)
	}
	_, err = OfferLayoutReader{}.Read([]TransceiverShape{{audio, sending}, {audio, sending}})
	if problemNumber(err) != 6509 {
		t.Fatal("two microphones were accepted")
	}
}

func TestPacketMarksReadAudioLevelAndKeyframes(t *testing.T) {
	packet := &rtp.Packet{Header: rtp.Header{Version: 2}}
	loud, _ := rtp.AudioLevelExtension{Level: 20}.Marshal()
	must(t, packet.SetExtension(3, loud))
	must(t, packet.SetExtension(4, []byte{0x20}))
	marks := PacketMarks{}
	if !marks.Loud(packet, 3) || marks.Loud(packet, 5) || !marks.Keyframe(packet, 4) || marks.Keyframe(packet, 3) {
		t.Fatal("marks were read wrong")
	}
	quiet := &rtp.Packet{Header: rtp.Header{Version: 2}}
	level, _ := rtp.AudioLevelExtension{Level: 100}.Marshal()
	must(t, quiet.SetExtension(3, level))
	if marks.Loud(quiet, 3) {
		t.Fatal("a quiet packet was loud")
	}
}

func TestKeyframesAreSeenInTheClearVp8Header(t *testing.T) {
	marks := PacketMarks{}
	cases := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{name: "keyframe start", payload: []byte{0x10, 0x00, 0xaa, 0xbb}, want: true},
		{name: "keyframe start with picture id", payload: []byte{0x90, 0x80, 0x81, 0x23, 0x00, 0xaa}, want: true},
		{name: "inter frame start", payload: []byte{0x10, 0x01, 0xaa}, want: false},
		{name: "keyframe continuation", payload: []byte{0x00, 0x00, 0xaa}, want: false},
		{name: "second partition", payload: []byte{0x11, 0x00, 0xaa}, want: false},
		{name: "too short", payload: []byte{0x10}, want: false},
	}
	for _, testCase := range cases {
		packet := &rtp.Packet{Header: rtp.Header{Version: 2}, Payload: testCase.payload}
		if marks.Keyframe(packet, 0) != testCase.want {
			t.Fatalf("%s: want %v", testCase.name, testCase.want)
		}
	}
}

func TestKeyframeRequestsAndGate(t *testing.T) {
	if !(KeyframeRequests{}).Asked([]rtcp.Packet{&rtcp.ReceiverReport{}, &rtcp.FullIntraRequest{}}) {
		t.Fatal("a full intra request was missed")
	}
	if (KeyframeRequests{}).Asked([]rtcp.Packet{&rtcp.ReceiverReport{}}) {
		t.Fatal("a report was taken as a request")
	}
	gate := NewKeyframeGate()
	now := time.Now()
	feed := FeedKey{DeviceId: "a"}
	if !gate.Allow(feed, now) || gate.Allow(feed, now.Add(100*time.Millisecond)) || !gate.Allow(feed, now.Add(time.Second)) {
		t.Fatal("the gate did not space requests")
	}
}

func TestSourceStateTracksActivity(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	state := NewSourceState("a", now)
	camera := FeedKey{DeviceId: "a", Kind: FeedKinds{}.Camera(), Rid: "h"}
	screen := FeedKey{DeviceId: "a", Kind: FeedKinds{}.Screen()}
	microphone := FeedKey{DeviceId: "a", Kind: FeedKinds{}.Microphone()}
	state.Track(microphone, 42)
	state.Heard(microphone, true, now)
	state.Heard(camera, false, now)
	state.Heard(screen, false, now)
	state.Heard(screen, false, now.Add(time.Second))
	snapshot := state.Snapshot(now.Add(time.Second))
	if !snapshot.HasMicrophone || !slices.Equal(snapshot.CameraRids, []string{"h"}) || !snapshot.ScreenSince.Equal(now) || !snapshot.LastSpokeAt.Equal(now) {
		t.Fatalf("snapshot %+v", snapshot)
	}
	later := state.Snapshot(now.Add(10 * time.Second))
	if later.HasCamera() || later.SharesScreen() {
		t.Fatalf("stale feeds are still active: %+v", later)
	}
	ssrc, found := state.Ssrc(microphone)
	if !found || ssrc != 42 {
		t.Fatal("the ssrc was lost")
	}
}

func TestBitrateHolderFallsBack(t *testing.T) {
	if NewBitrateHolder(123).Bitrate() != 123 {
		t.Fatal("no fallback")
	}
}

func TestExtensionIdsFindsTheId(t *testing.T) {
	extensions := []webrtc.RTPHeaderExtensionParameter{{URI: "a", ID: 2}, {URI: "b", ID: 5}}
	if (ExtensionIds{}).Find(extensions, "b") != 5 || (ExtensionIds{}).Find(extensions, "c") != 0 {
		t.Fatal("wrong id")
	}
}
