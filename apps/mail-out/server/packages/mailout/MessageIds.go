package mailout

import "strings"

type MessageIds struct{}

func (ids MessageIds) Header(item Item) string {
	id := ids.Bare(item.MessageId)
	if strings.Contains(id, "@") {
		return id
	}
	return id + "@" + item.DomainId
}

func (MessageIds) Bare(id string) string {
	return strings.Trim(strings.TrimSpace(id), "<>")
}

func (ids MessageIds) BareAll(list []string) []string {
	bare := []string{}
	for _, id := range list {
		bare = append(bare, ids.Bare(id))
	}
	return bare
}
