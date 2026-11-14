package media

import (
	"time"

	"github.com/pion/webrtc/v4"
)

type SlotKind struct {
	Name  string
	Codec webrtc.RTPCodecCapability
	Wait  time.Duration
	Step  uint32
}
