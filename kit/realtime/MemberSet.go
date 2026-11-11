package realtime

import "slices"

type MemberSet struct{}

func (MemberSet) Sorted(memberIds []string) []string {
	sorted := slices.Sorted(slices.Values(memberIds))
	return append([]string{}, slices.Compact(sorted)...)
}

func (set MemberSet) With(memberIds []string, memberId string) []string {
	return set.Sorted(append(slices.Clone(memberIds), memberId))
}

func (set MemberSet) Without(memberIds []string, memberId string) []string {
	kept := slices.DeleteFunc(slices.Clone(memberIds), func(member string) bool { return member == memberId })
	return set.Sorted(kept)
}

func (set MemberSet) Same(first []string, second []string) bool {
	return slices.Equal(set.Sorted(first), set.Sorted(second))
}
