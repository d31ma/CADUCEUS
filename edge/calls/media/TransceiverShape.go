package media

import "github.com/pion/webrtc/v4"

type TransceiverShape struct {
	Kind      webrtc.RTPCodecType
	Direction webrtc.RTPTransceiverDirection
}
