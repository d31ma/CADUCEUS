package blobs

type Inspection struct {
	Size        int64
	Sha256      string
	SniffedType string
}
