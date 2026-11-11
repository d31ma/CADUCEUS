package reception

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"slices"
	"strings"
)

type RawKey struct{}

func (key RawKey) For(raw []byte, envelopeFrom string, group DomainGroup) string {
	hash := sha256.New()
	hash.Write(raw)
	hash.Write([]byte(key.separator()))
	hash.Write([]byte(envelopeFrom))
	hash.Write([]byte(key.separator()))
	hash.Write([]byte(strings.Join(slices.Sorted(slices.Values(group.Recipients)), key.listSeparator())))
	return path.Join(group.DomainId, key.folder(), hex.EncodeToString(hash.Sum(nil))+key.extension())
}

func (RawKey) separator() string {
	return "\x00"
}

func (RawKey) listSeparator() string {
	return "\n"
}

func (RawKey) folder() string {
	return "inbound"
}

func (RawKey) extension() string {
	return ".eml"
}
