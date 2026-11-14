package media

import (
	"time"

	"github.com/pion/webrtc/v4"
)

type Codecs struct{}

func (Codecs) Opus() webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"},
		PayloadType:        111,
	}
}

func (Codecs) Vp8() webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, RTCPFeedback: []webrtc.RTCPFeedback{
			{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"}, {Type: "transport-cc"},
		}},
		PayloadType: 96,
	}
}

func (codecs Codecs) VideoSlot() SlotKind {
	return SlotKind{Name: "video", Codec: codecs.Vp8().RTPCodecCapability, Wait: 300 * time.Millisecond, Step: 3000}
}

func (codecs Codecs) AudioSlot() SlotKind {
	return SlotKind{Name: "audio", Codec: codecs.Opus().RTPCodecCapability, Step: 960}
}
