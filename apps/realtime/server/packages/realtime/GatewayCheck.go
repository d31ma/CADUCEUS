package realtime

import (
	"crypto/sha256"
	"crypto/subtle"
)

type GatewayCheck struct {
	Secret string
}

func (check GatewayCheck) Allowed(header string) bool {
	given := sha256.Sum256([]byte(header))
	return check.matches(given[:])
}

func (check GatewayCheck) matches(given []byte) bool {
	if len(check.Secret) < check.minimumLength() {
		return false
	}
	expected := sha256.Sum256([]byte(check.Secret))
	return subtle.ConstantTimeCompare(expected[:], given) == 1
}

func (GatewayCheck) minimumLength() int {
	return 32
}
