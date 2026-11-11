package sending

import (
	"regexp"
	"strings"

	"mesail.dev/packages/core"
)

type Mentions struct {
	members core.MemberStore
}

func NewMentions(members core.MemberStore) Mentions {
	return Mentions{members: members}
}

func (mentions Mentions) of(domainId string, body string, readers []string, authorId string) ([]string, error) {
	allowed := mentions.allowed(readers, authorId)
	found := []string{}
	for _, name := range mentions.names(body) {
		member, known, err := mentions.members.ByLocalPart(domainId, name)
		if err != nil {
			return nil, Failures{}.store(err)
		}
		found = append(found, mentions.ifAllowed(member, known, allowed)...)
	}
	return found, nil
}

func (Mentions) allowed(readers []string, authorId string) map[string]bool {
	allowed := map[string]bool{}
	for _, memberId := range readers {
		allowed[memberId] = true
	}
	delete(allowed, authorId)
	return allowed
}

func (Mentions) ifAllowed(member core.Member, known bool, allowed map[string]bool) []string {
	if !known || !allowed[member.MemberId] {
		return nil
	}
	delete(allowed, member.MemberId)
	return []string{member.MemberId}
}

func (mentions Mentions) names(body string) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, match := range regexp.MustCompile(mentions.pattern()).FindAllStringSubmatch(strings.ToLower(body), -1) {
		names = append(names, mentions.ifUnseen(strings.TrimRight(match[1], mentions.trailing()), seen)...)
	}
	return names
}

func (Mentions) ifUnseen(name string, seen map[string]bool) []string {
	if name == "" || seen[name] {
		return nil
	}
	seen[name] = true
	return []string{name}
}

func (Mentions) pattern() string {
	return `(?:^|[^a-z0-9._-])@([a-z0-9._-]+)`
}

func (Mentions) trailing() string {
	return "."
}
