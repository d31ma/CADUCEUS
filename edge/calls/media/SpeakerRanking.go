package media

import (
	"cmp"
	"slices"
	"strings"
)

type SpeakerRanking struct{}

func (ranking SpeakerRanking) Order(sources []SourceSnapshot, viewer string) []SourceSnapshot {
	others := ranking.without(sources, viewer)
	slices.SortStableFunc(others, ranking.compare)
	return others
}

func (SpeakerRanking) without(sources []SourceSnapshot, viewer string) []SourceSnapshot {
	return slices.DeleteFunc(slices.Clone(sources), func(source SourceSnapshot) bool {
		return source.DeviceId == viewer
	})
}

func (SpeakerRanking) compare(first SourceSnapshot, second SourceSnapshot) int {
	return cmp.Or(
		second.LastSpokeAt.Compare(first.LastSpokeAt),
		first.JoinedAt.Compare(second.JoinedAt),
		strings.Compare(first.DeviceId, second.DeviceId),
	)
}
