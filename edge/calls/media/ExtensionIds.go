package media

import (
	"slices"

	"github.com/pion/webrtc/v4"
)

type ExtensionIds struct{}

func (ExtensionIds) Find(extensions []webrtc.RTPHeaderExtensionParameter, uri string) uint8 {
	index := slices.IndexFunc(extensions, func(extension webrtc.RTPHeaderExtensionParameter) bool {
		return extension.URI == uri
	})
	if index < 0 {
		return 0
	}
	return uint8(extensions[index].ID)
}

func (ExtensionIds) FrameMarkingUri() string {
	return "http://tools.ietf.org/html/draft-ietf-avtext-framemarking-07"
}
