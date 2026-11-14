package media

import (
	"slices"

	"github.com/pion/rtcp"
)

type KeyframeRequests struct{}

func (requests KeyframeRequests) Asked(packets []rtcp.Packet) bool {
	return slices.ContainsFunc(packets, requests.isRequest)
}

func (KeyframeRequests) isRequest(packet rtcp.Packet) bool {
	switch packet.(type) {
	case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
		return true
	}
	return false
}
