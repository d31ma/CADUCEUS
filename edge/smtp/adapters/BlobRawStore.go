package adapters

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"mesail.dev/packages/blobs"
)

type BlobRawStore struct {
	store blobs.Store
}

func NewBlobRawStore(store blobs.Store) BlobRawStore {
	return BlobRawStore{store: store}
}

func (raw BlobRawStore) Put(_ context.Context, key string, content []byte) error {
	blobKey, err := raw.checked(raw.split(key))
	if err != nil {
		return err
	}
	return raw.store.Write(blobKey, bytes.NewReader(content), int64(len(content)))
}

func (BlobRawStore) split(key string) blobs.Key {
	domainId, name, _ := strings.Cut(key, "/")
	return blobs.Key{DomainId: domainId, Name: name}
}

func (BlobRawStore) checked(key blobs.Key) (blobs.Key, error) {
	err := blobs.KeyRules{}.Check(key)
	if err != nil {
		return blobs.Key{}, errors.Join(InvalidKey{}, err)
	}
	return key, nil
}
