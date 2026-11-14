package media

import (
	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/failures"
)

type OfferLayoutReader struct{}

func (reader OfferLayoutReader) Read(shapes []TransceiverShape) (OfferLayout, error) {
	microphones := reader.indexes(shapes, TransceiverShape{Kind: webrtc.RTPCodecTypeAudio, Direction: webrtc.RTPTransceiverDirectionRecvonly})
	cameras := reader.indexes(shapes, TransceiverShape{Kind: webrtc.RTPCodecTypeVideo, Direction: webrtc.RTPTransceiverDirectionRecvonly})
	videoSlots := reader.indexes(shapes, TransceiverShape{Kind: webrtc.RTPCodecTypeVideo, Direction: webrtc.RTPTransceiverDirectionSendonly})
	audioSlots := reader.indexes(shapes, TransceiverShape{Kind: webrtc.RTPCodecTypeAudio, Direction: webrtc.RTPTransceiverDirectionSendonly})
	err := reader.withinLimits(len(microphones), len(cameras), len(videoSlots), len(audioSlots))
	if err != nil {
		return OfferLayout{}, err
	}
	return OfferLayout{
		Microphone: microphones,
		Camera:     reader.head(cameras, 1),
		Screen:     reader.tail(cameras, 1),
		ScreenSlot: reader.head(videoSlots, 1),
		VideoSlots: reader.tail(videoSlots, 1),
		AudioSlots: audioSlots,
	}, nil
}

func (OfferLayoutReader) indexes(shapes []TransceiverShape, wanted TransceiverShape) []int {
	indexes := []int{}
	for index, shape := range shapes {
		indexes = append(indexes, OfferLayoutReader{}.matching(index, shape, wanted)...)
	}
	return indexes
}

func (OfferLayoutReader) matching(index int, shape TransceiverShape, wanted TransceiverShape) []int {
	if shape != wanted {
		return nil
	}
	return []int{index}
}

func (OfferLayoutReader) withinLimits(microphones int, cameras int, videoSlots int, audioSlots int) error {
	limits := Limits{}
	if microphones > 1 || cameras > 2 || videoSlots > limits.VideosPerReceiver()+1 || audioSlots > limits.People()-1 {
		return failures.Problems{}.InvalidOffer()
	}
	return nil
}

func (OfferLayoutReader) head(indexes []int, count int) []int {
	return indexes[:min(count, len(indexes))]
}

func (OfferLayoutReader) tail(indexes []int, count int) []int {
	return indexes[min(count, len(indexes)):]
}
