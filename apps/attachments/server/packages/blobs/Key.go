package blobs

type Key struct {
	DomainId string
	Name     string
}

func (key Key) Path() string {
	return key.DomainId + "/" + key.Name
}
