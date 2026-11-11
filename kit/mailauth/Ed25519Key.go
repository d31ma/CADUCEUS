package mailauth

import "crypto/ed25519"

type ed25519Key struct {
	public ed25519.PublicKey
}

func (ed25519Key) kind() string {
	return dkimAlgorithms{}.ed25519()
}

func (key ed25519Key) verify(digest []byte, signature []byte) bool {
	return ed25519.Verify(key.public, digest, signature)
}
