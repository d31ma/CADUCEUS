package mailout

import "strings"

type MessageIds struct{}

func (MessageIds) atSign() string { return "@" }

func (ids MessageIds) Header(item Item) string {
	return ids.withDomain(ids.Bare(item.MessageId), item.DomainId)
}

func (MessageIds) Bare(id string) string {
	return strings.Trim(strings.TrimSpace(id), "<>")
}

func (ids MessageIds) BareAll(list []string) []string {
	return ids.bareList(list)
}

func (ids MessageIds) withDomain(id string, domainId string) string {
	if strings.Contains(id, ids.atSign()) {
		return id
	}
	return id + ids.atSign() + domainId
}

func (ids MessageIds) bareList(list []string) []string {
	bare := []string{}
	for _, id := range list {
		bare = append(bare, ids.Bare(id))
	}
	return bare
}
