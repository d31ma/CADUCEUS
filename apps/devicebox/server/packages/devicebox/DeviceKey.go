package devicebox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
)

type DeviceKey struct{}

func (key DeviceKey) Of(deviceId string) string {
	digest := sha256.Sum256([]byte(deviceId))
	return key.prefix() + hex.EncodeToString(digest[:key.digestBytes()])
}

func (key DeviceKey) Bucket(deviceKey string) string {
	value, _ := strconv.ParseUint(deviceKey[1:2], 16, 8)
	return fmt.Sprintf("b%d", value%key.bucketCount())
}

func (key DeviceKey) Valid(deviceKey string) bool {
	return regexp.MustCompile(`^d[0-9a-f]{24}$`).MatchString(deviceKey)
}

func (DeviceKey) prefix() string {
	return "d"
}

func (DeviceKey) digestBytes() int {
	return 12
}

func (DeviceKey) bucketCount() uint64 {
	return 8
}
