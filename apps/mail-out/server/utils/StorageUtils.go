import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/outbox"
)

type StorageUtils struct{}

func (StorageUtils) OutboundQueue() string { return "outbound-queue" }

func (StorageUtils) CollectionName(collection string, domainId string) string {
	return collections.CollectionName{}.ForDomain(collection, domainId)
}

func (utils StorageUtils) QueueRef(item OutboundItem) outbox.Ref {
	return outbox.Ref{Collection: utils.CollectionName(utils.OutboundQueue(), item.DomainId), Id: item.Id}
}

func (StorageUtils) hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (utils StorageUtils) AddressHash(address string) string {
	return utils.hash(strings.ToLower(strings.TrimSpace(address)))
}
