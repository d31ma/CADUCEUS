package blobs

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

type Inspector struct{}

func (inspector Inspector) Inspect(reader io.Reader, limit int64) (Inspection, error) {
	hash := sha256.New()
	head := &HeadBuffer{capacity: inspector.sniffLength()}
	size, err := io.Copy(io.MultiWriter(hash, head), io.LimitReader(reader, limit+1))
	if err != nil {
		return Inspection{}, err
	}
	inspection := Inspection{Size: size, Sha256: hex.EncodeToString(hash.Sum(nil)), SniffedType: http.DetectContentType(head.Bytes())}
	return inspection, inspector.checkLimit(size, limit)
}

func (Inspector) checkLimit(size int64, limit int64) error {
	if size > limit {
		return TooLarge{Limit: limit}
	}
	return nil
}

func (Inspector) sniffLength() int {
	return 512
}
