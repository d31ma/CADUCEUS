package mailin

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"mesail.dev/packages/blobs"
)

type BlobKeys struct{}

func (BlobKeys) FromPath(path string) blobs.Key {
	domainId, name, _ := strings.Cut(path, "/")
	return blobs.Key{DomainId: domainId, Name: name}
}

func (keys BlobKeys) HandOffCopy(recipientDomain string, senderPath string) blobs.Key {
	return blobs.Key{DomainId: recipientDomain, Name: "inbound-" + keys.hashed(senderPath)}
}

func (keys BlobKeys) Attachment(domainId string, rawPath string, index int) blobs.Key {
	return blobs.Key{DomainId: domainId, Name: "attachment-" + keys.hashed(rawPath+"#"+strconv.Itoa(index))}
}

func (BlobKeys) hashed(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}
