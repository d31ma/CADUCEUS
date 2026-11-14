import (
	"os"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/realtime"
)

// @Service
type CallKeysService struct {
	// @Needs
	failures *ErrorMappingService
}

func (service *CallKeysService) Keys() (CallKeys, error) {
	set, failed := realtime.DeskFactory{Environment: os.Getenv}.CallSigner().KeySet()
	return CallKeys{Keys: service.keys(set)}, service.failures.CallKeysError(failed)
}

func (service *CallKeysService) keys(set oidc.KeySet) []CallKey {
	keys := []CallKey{}
	for _, key := range set.Keys {
		keys = append(keys, CallKey{KeyType: key.KeyType, KeyId: key.KeyId, Algorithm: key.Algorithm, Use: key.Use, Curve: key.Curve, X: key.X, Y: key.Y})
	}
	return keys
}
