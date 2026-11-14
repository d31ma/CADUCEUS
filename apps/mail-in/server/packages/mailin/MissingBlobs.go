package mailin

import (
	"cmp"
	"io"
	"time"

	"mesail.dev/packages/blobs"
)

type MissingBlobs struct {
	Problem string
}

func (missing MissingBlobs) SignUpload(key blobs.Key, upload blobs.UploadTerms, expiresAt time.Time) (string, error) {
	return "", missing.failure()
}

func (missing MissingBlobs) SignDownload(key blobs.Key, download blobs.DownloadTerms, expiresAt time.Time) (string, error) {
	return "", missing.failure()
}

func (missing MissingBlobs) Read(key blobs.Key) (io.ReadCloser, error) {
	return nil, missing.failure()
}

func (missing MissingBlobs) Write(key blobs.Key, body io.Reader, size int64) error {
	return missing.failure()
}

func (missing MissingBlobs) Copy(from blobs.Key, to blobs.Key) error {
	return missing.failure()
}

func (missing MissingBlobs) Delete(key blobs.Key) error {
	return missing.failure()
}

func (missing MissingBlobs) Reason() string {
	return cmp.Or(missing.Problem, "blob store settings")
}

func (missing MissingBlobs) failure() error {
	return Unavailable{Operation: missing.Reason()}
}
