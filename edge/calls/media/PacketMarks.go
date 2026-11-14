package media

import (
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

type PacketMarks struct{}

func (marks PacketMarks) Loud(packet *rtp.Packet, id uint8) bool {
	level := rtp.AudioLevelExtension{}
	err := level.Unmarshal(packet.GetExtension(id))
	return marks.loud(err, level)
}

func (marks PacketMarks) Keyframe(packet *rtp.Packet, frameMarkingId uint8) bool {
	return marks.independent(packet.GetExtension(frameMarkingId)) || marks.vp8Keyframe(packet.Payload)
}

func (marks PacketMarks) loud(err error, level rtp.AudioLevelExtension) bool {
	return err == nil && level.Level <= marks.quietestSpeech()
}

func (marks PacketMarks) independent(raw []byte) bool {
	return len(raw) > 0 && raw[0]&marks.independentBit() != 0
}

func (marks PacketMarks) vp8Keyframe(payload []byte) bool {
	vp8 := codecs.VP8Packet{}
	_, err := vp8.Unmarshal(payload)
	return err == nil && vp8.S == 1 && vp8.PID == 0 && len(vp8.Payload) > 0 && vp8.Payload[0]&marks.interFrameBit() == 0
}

func (PacketMarks) quietestSpeech() uint8 { return 55 }
func (PacketMarks) independentBit() byte  { return 0x20 }
func (PacketMarks) interFrameBit() byte   { return 0x01 }
