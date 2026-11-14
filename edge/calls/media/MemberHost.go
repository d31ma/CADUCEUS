package media

import (
	"time"

	"github.com/pion/rtp"
)

type MemberHost interface {
	Deliver(feed FeedKey, packet *rtp.Packet, frame FrameStart, now time.Time)
	RequestKeyframe(feed FeedKey)
	Gone(member *Member)
}
