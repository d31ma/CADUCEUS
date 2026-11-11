package blobs

type BadSignature struct{}

func (BadSignature) Error() string {
	return "the blob link signature is not valid"
}
