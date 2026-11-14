package media

import (
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/admission"
)

type TrackReader struct {
	track          *webrtc.TrackRemote
	feed           FeedKey
	source         *SourceState
	host           MemberHost
	clock          admission.Clock
	audioLevelId   uint8
	frameMarkingId uint8
	lastTimestamp  uint32
}

func NewTrackReader(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, feed FeedKey, member *Member) *TrackReader {
	extensions := receiver.GetParameters().HeaderExtensions
	return &TrackReader{
		track:          track,
		feed:           feed,
		source:         member.source,
		host:           member.host,
		clock:          member.clock,
		audioLevelId:   ExtensionIds{}.Find(extensions, sdp.AudioLevelURI),
		frameMarkingId: ExtensionIds{}.Find(extensions, ExtensionIds{}.FrameMarkingUri()),
	}
}

func (reader *TrackReader) Run() {
	reader.loop()
}

func (reader *TrackReader) loop() {
	for reader.next() {
	}
}

func (reader *TrackReader) next() bool {
	packet, _, err := reader.track.ReadRTP()
	if err != nil {
		return false
	}
	now := reader.clock.Now()
	reader.source.Heard(reader.feed, PacketMarks{}.Loud(packet, reader.audioLevelId), now)
	frame := FrameStart{Independent: PacketMarks{}.Keyframe(packet, reader.frameMarkingId), New: packet.Timestamp != reader.lastTimestamp}
	reader.lastTimestamp = packet.Timestamp
	reader.host.Deliver(reader.feed, packet, frame, now)
	return true
}
