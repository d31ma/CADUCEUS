package sending

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Hashes struct{}

func (Hashes) hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (hashes Hashes) sendKey(memberId string, clientMessageId string) string {
	return hashes.hex(memberId + hashes.separator() + clientMessageId)
}

func (hashes Hashes) participantKey(memberIds []string) string {
	return hashes.hex(strings.Join(memberIds, hashes.separator()))
}

func (hashes Hashes) threadKey(header string) string {
	return hashes.hex(strings.ToLower(header))
}

func (Hashes) separator() string {
	return "\n"
}
