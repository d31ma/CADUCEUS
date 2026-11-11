package reading

import (
	"crypto/sha256"
	"encoding/hex"
)

type reactionKeys struct{}

func (keys reactionKeys) of(messageId string, memberId string, reaction string) string {
	sum := sha256.Sum256([]byte(messageId + keys.separator() + memberId + keys.separator() + reaction))
	return hex.EncodeToString(sum[:])
}

func (reactionKeys) separator() string {
	return "\n"
}
