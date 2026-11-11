package adapters

type InvalidKey struct{}

func (InvalidKey) Error() string {
	return "the raw message key is not a safe relative path"
}
