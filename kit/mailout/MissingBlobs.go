package mailout

import (
	"errors"
	"io"
	"time"

	"mesail.dev/packages/blobs"
)

type MissingBlobs struct{}

func (MissingBlobs) failure() error { return errors.New("mail-out has no blob folder") }

func (store MissingBlobs) SignUpload(key blobs.Key, upload blobs.UploadTerms, expiresAt time.Time) (string, error) {
	return "", store.failure()
}

func (store MissingBlobs) SignDownload(key blobs.Key, download blobs.DownloadTerms, expiresAt time.Time) (string, error) {
	return "", store.failure()
}

func (store MissingBlobs) Read(key blobs.Key) (io.ReadCloser, error) {
	return nil, store.failure()
}

func (store MissingBlobs) Write(key blobs.Key, body io.Reader, size int64) error {
	return store.failure()
}

func (store MissingBlobs) Copy(from blobs.Key, to blobs.Key) error {
	return store.failure()
}

func (store MissingBlobs) Delete(key blobs.Key) error {
	return store.failure()
}
