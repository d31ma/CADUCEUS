package backups

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"mesail.dev/packages/blobs"
)

type BlobNames struct{}

func (names BlobNames) New(domainId string, personId string, version int64) blobs.Key {
	name := names.prefix() + names.personKey(personId) + "-" + strconv.FormatInt(version, 10) + "-" + rand.Text()
	return blobs.Key{DomainId: domainId, Name: name}
}

func (BlobNames) Key(path string) blobs.Key {
	domainId, name, _ := strings.Cut(path, "/")
	return blobs.Key{DomainId: domainId, Name: name}
}

func (names BlobNames) personKey(personId string) string {
	digest := sha256.Sum256([]byte(personId))
	return hex.EncodeToString(digest[:names.keyBytes()])
}

func (BlobNames) prefix() string {
	return "backup-"
}

func (BlobNames) keyBytes() int {
	return 12
}
