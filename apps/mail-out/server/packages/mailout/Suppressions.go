package mailout

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Suppressions struct {
	Store DocumentStore
}

func (Suppressions) base() string { return "suppressions" }

func (Suppressions) Hash(address string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(address))))
	return hex.EncodeToString(sum[:])
}

func (list Suppressions) Contains(domainId string, address string) (bool, error) {
	collection, err := list.Store.Ready(list.base(), domainId)
	if err != nil {
		return false, err
	}
	return list.Store.Exists(collection, map[string]any{"addressHash": list.Hash(address)})
}

func (list Suppressions) Add(suppression Suppression) error {
	known, err := list.Contains(suppression.DomainId, suppression.Address)
	if err != nil || known {
		return err
	}
	collection, err := list.Store.Ready(list.base(), suppression.DomainId)
	if err != nil {
		return err
	}
	suppression.AddressHash = list.Hash(suppression.Address)
	_, err = list.Store.Documents.Insert(collection, suppression)
	return err
}
