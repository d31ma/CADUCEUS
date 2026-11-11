package realtime

import "strings"

type MemberIdList struct{}

func (list MemberIdList) Parse(text string) []string {
	return MemberSet{}.Without(list.trimmed(strings.Split(text, ",")), "")
}

func (MemberIdList) trimmed(parts []string) []string {
	memberIds := []string{}
	for _, part := range parts {
		memberIds = append(memberIds, strings.TrimSpace(part))
	}
	return memberIds
}
