package media

import (
	"slices"
)

type ReceiverPlanner struct{}

func (planner ReceiverPlanner) Plan(viewer ViewerState, sources []SourceSnapshot) ReceiverPlan {
	ranked := SpeakerRanking{}.Order(sources, viewer.DeviceId)
	cameras := planner.top(planner.withCamera(ranked), len(viewer.VideoShown))
	video := SlotAssignment{}.Assign(viewer.VideoShown, planner.ids(cameras))
	audio := SlotAssignment{}.Assign(viewer.AudioShown, planner.ids(planner.top(planner.withMicrophone(ranked), len(viewer.AudioShown))))
	sharer := planner.sharer(ranked)
	return ReceiverPlan{
		Video:   planner.videoFeeds(video, cameras, LayerPlan{}.Choose(viewer.Bitrate, len(cameras))),
		Screen:  planner.screenFeed(sharer, viewer.ScreenSlot),
		Audio:   planner.audioFeeds(audio),
		Message: SlotMessage{Type: "slots", Video: video, Screen: sharer.DeviceId, Audio: audio, Order: planner.ids(ranked)},
	}
}

func (ReceiverPlanner) withCamera(ranked []SourceSnapshot) []SourceSnapshot {
	return slices.DeleteFunc(slices.Clone(ranked), func(source SourceSnapshot) bool { return !source.HasCamera() })
}

func (ReceiverPlanner) withMicrophone(ranked []SourceSnapshot) []SourceSnapshot {
	return slices.DeleteFunc(slices.Clone(ranked), func(source SourceSnapshot) bool { return !source.HasMicrophone })
}

func (ReceiverPlanner) top(sources []SourceSnapshot, count int) []SourceSnapshot {
	return sources[:min(count, len(sources))]
}

func (ReceiverPlanner) ids(sources []SourceSnapshot) []string {
	ids := make([]string, len(sources))
	for index, source := range sources {
		ids[index] = source.DeviceId
	}
	return ids
}

func (planner ReceiverPlanner) videoFeeds(video []string, cameras []SourceSnapshot, qualities []int) []FeedKey {
	feeds := make([]FeedKey, len(video))
	for index, deviceId := range video {
		feeds[index] = planner.videoFeed(deviceId, cameras, qualities)
	}
	return feeds
}

func (ReceiverPlanner) videoFeed(deviceId string, cameras []SourceSnapshot, qualities []int) FeedKey {
	index := slices.IndexFunc(cameras, func(source SourceSnapshot) bool { return source.DeviceId == deviceId })
	if index < 0 {
		return FeedKey{}
	}
	return FeedKey{DeviceId: deviceId, Kind: FeedKinds{}.Camera(), Rid: RidChoice{}.For(qualities[index], cameras[index].CameraRids)}
}

func (ReceiverPlanner) sharer(ranked []SourceSnapshot) SourceSnapshot {
	sharing := slices.DeleteFunc(slices.Clone(ranked), func(source SourceSnapshot) bool { return !source.SharesScreen() })
	if len(sharing) == 0 {
		return SourceSnapshot{}
	}
	return slices.MinFunc(sharing, func(first SourceSnapshot, second SourceSnapshot) int {
		return first.ScreenSince.Compare(second.ScreenSince)
	})
}

func (ReceiverPlanner) screenFeed(sharer SourceSnapshot, hasSlot bool) FeedKey {
	if !hasSlot || sharer.DeviceId == "" {
		return FeedKey{}
	}
	return FeedKey{DeviceId: sharer.DeviceId, Kind: FeedKinds{}.Screen()}
}

func (planner ReceiverPlanner) audioFeeds(audio []string) []FeedKey {
	feeds := make([]FeedKey, len(audio))
	for index, deviceId := range audio {
		feeds[index] = planner.audioFeed(deviceId)
	}
	return feeds
}

func (ReceiverPlanner) audioFeed(deviceId string) FeedKey {
	if deviceId == "" {
		return FeedKey{}
	}
	return FeedKey{DeviceId: deviceId, Kind: FeedKinds{}.Microphone()}
}
