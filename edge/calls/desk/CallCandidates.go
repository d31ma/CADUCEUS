package desk

import "github.com/pion/webrtc/v4"

type CallCandidates struct {
	Candidates []webrtc.ICECandidateInit `json:"candidates"`
}
