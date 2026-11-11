package blobs

import (
	"io"
	"time"
)

type Store interface {
	SignUpload(key Key, upload UploadTerms, expiresAt time.Time) (string, error)
	SignDownload(key Key, download DownloadTerms, expiresAt time.Time) (string, error)
	Read(key Key) (io.ReadCloser, error)
	Write(key Key, body io.Reader, size int64) error
	Copy(from Key, to Key) error
	Delete(key Key) error
}
