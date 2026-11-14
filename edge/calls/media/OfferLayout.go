package media

import "slices"

type OfferLayout struct {
	Microphone []int
	Camera     []int
	Screen     []int
	VideoSlots []int
	ScreenSlot []int
	AudioSlots []int
}

func (layout OfferLayout) KindAt(index int) FeedKind {
	switch {
	case slices.Contains(layout.Microphone, index):
		return FeedKinds{}.Microphone()
	case slices.Contains(layout.Camera, index):
		return FeedKinds{}.Camera()
	case slices.Contains(layout.Screen, index):
		return FeedKinds{}.Screen()
	}
	return ""
}
