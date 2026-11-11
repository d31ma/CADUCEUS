package blobs

type Expired struct{}

func (Expired) Error() string {
	return "the blob link has expired"
}
