import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// @Service
type SessionProofService struct {
	// @Needs
	secureRandom SecureRandom
}

func (proof *SessionProofService) NewKey() (SessionProofKey, error) {
	verifier, verifierFailure := proof.randomText()
	state, stateFailure := proof.randomText()
	key := SessionProofKey{Verifier: verifier, Challenge: proof.challenge(verifier), State: state}
	return key, errors.Join(verifierFailure, stateFailure)
}

func (proof *SessionProofService) randomText() (string, error) {
	bytes, failure := proof.secureRandom.Bytes(proof.randomByteCount())
	return base64.RawURLEncoding.EncodeToString(bytes), failure
}

func (proof *SessionProofService) challenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (proof *SessionProofService) randomByteCount() int {
	return 32
}
