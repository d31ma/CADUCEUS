package media

import "github.com/pion/webrtc/v4"

type PeerParts struct {
	Peer    *webrtc.PeerConnection
	Api     *webrtc.API
	Bitrate *BitrateHolder
}
