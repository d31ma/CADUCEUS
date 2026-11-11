package mailin

import (
	"io"
	"time"

	"mesail.dev/packages/blobs"
)

type MissingBlobs struct{}

func (MissingBlobs) SignUpload(key blobs.Key, upload blobs.UploadTerms, expiresAt time.Time) (string, error) {
	return "", MissingBlobs{}.failure()
}

func (MissingBlobs) SignDownload(key blobs.Key, download blobs.DownloadTerms, expiresAt time.Time) (string, error) {
	return "", MissingBlobs{}.failure()
}

func (MissingBlobs) Read(key blobs.Key) (io.ReadCloser, error) {
	return nil, MissingBlobs{}.failure()
}

func (MissingBlobs) Write(key blobs.Key, body io.Reader, size int64) error {
	return MissingBlobs{}.failure()
}

func (MissingBlobs) Copy(from blobs.Key, to blobs.Key) error {
	return MissingBlobs{}.failure()
}

func (MissingBlobs) Delete(key blobs.Key) error {
	return MissingBlobs{}.failure()
}

func (MissingBlobs) failure() error {
	return Unavailable{Operation: "blob store settings"}
}
