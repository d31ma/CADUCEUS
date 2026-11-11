import "strings"

type MessageIdUtils struct{}

func (MessageIdUtils) at() string { return "@" }

func (utils MessageIdUtils) Header(item OutboundItem) string {
	return utils.qualified(utils.Bare(item.MessageId), item.DomainId)
}

func (MessageIdUtils) Bare(id string) string {
	return strings.Trim(strings.TrimSpace(id), "<>")
}

func (utils MessageIdUtils) BareAll(ids []string) []string {
	return utils.bareAll(ids)
}

func (utils MessageIdUtils) qualified(id string, domainId string) string {
	if strings.Contains(id, utils.at()) {
		return id
	}
	return id + utils.at() + domainId
}

func (utils MessageIdUtils) bareAll(ids []string) []string {
	bare := []string{}
	for _, id := range ids {
		bare = append(bare, utils.Bare(id))
	}
	return bare
}
