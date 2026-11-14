package media

import "time"

type PeerOptions struct {
	PublicAddresses     []string
	IncludeLoopback     bool
	DisconnectedTimeout time.Duration
	FailedTimeout       time.Duration
	KeepAliveInterval   time.Duration
	StartBitrate        int
}
