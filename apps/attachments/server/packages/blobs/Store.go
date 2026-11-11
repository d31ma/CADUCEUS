package blobs

import (
	"io"
	"time"
)

type Store interface {
	SignUpload(key Key, upload UploadTerms, expiresAt time.Time) (string, error)
	SignDownload(key Key, download DownloadTerms, expiresAt time.Time) (string, error)
	Read(key Key) (io.ReadCloser, error)
	Delete(key Key) error
}
