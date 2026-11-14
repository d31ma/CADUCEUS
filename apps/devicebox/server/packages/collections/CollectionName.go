package collections

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type CollectionName struct{}

func (name CollectionName) ForDomain(base string, domainId string) string {
	return base + "-" + name.DomainKey(domainId)
}

func (name CollectionName) ForDomainMonth(base string, domainId string, yearMonth string) string {
	return name.ForDomain(base, domainId) + "-" + yearMonth
}

func (name CollectionName) DomainKey(domainId string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(domainId))))
	return hex.EncodeToString(digest[:name.keyBytes()])
}

func (CollectionName) keyBytes() int {
	return 8
}
