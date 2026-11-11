package mailauth

type signatureKey interface {
	kind() string
	verify(digest []byte, signature []byte) bool
}
