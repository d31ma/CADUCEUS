package devicebox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
)

type ContentHash struct{}

func (ContentHash) Of(put CheckedPut) string {
	receivers := slices.Clone(put.ToDeviceIds)
	slices.Sort(receivers)
	encoded, _ := json.Marshal([]any{put.SenderDeviceId, receivers, put.Kind, put.Ciphertext})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
