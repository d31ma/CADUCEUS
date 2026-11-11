package dkim

type SecretSource interface {
	Secret(name string) ([]byte, error)
}
