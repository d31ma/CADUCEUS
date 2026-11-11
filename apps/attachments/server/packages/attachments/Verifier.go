package attachments

import (
	"cmp"
	"errors"

	"mesail.dev/packages/blobs"
)

type Verifier struct{}

func (verifier Verifier) Verify(store blobs.Store, key blobs.Key, record Record) (string, error) {
	reader, err := store.Read(key)
	if err != nil {
		return "", verifier.readFailure(err)
	}
	defer reader.Close()
	inspection, err := blobs.Inspector{}.Inspect(reader, record.Size)
	if err != nil {
		return "", verifier.inspectFailure(err)
	}
	err = cmp.Or(verifier.checkSize(record, inspection), verifier.checkSha256(record, inspection))
	if err != nil {
		return "", err
	}
	return verifier.state(record, inspection), nil
}

func (Verifier) readFailure(cause error) error {
	if errors.As(cause, &blobs.NotFound{}) {
		return Conflict{Field: "file", Reason: "The file has not been uploaded yet."}
	}
	return Unavailable{Operation: "read"}
}

func (Verifier) inspectFailure(cause error) error {
	if errors.As(cause, &blobs.TooLarge{}) {
		return cause
	}
	return Unavailable{Operation: "inspect"}
}

func (Verifier) checkSize(record Record, inspection blobs.Inspection) error {
	if inspection.Size != record.Size {
		return Conflict{Field: "size", Reason: "The uploaded file is smaller than the size you gave."}
	}
	return nil
}

func (Verifier) checkSha256(record Record, inspection blobs.Inspection) error {
	if inspection.Sha256 != record.Sha256 {
		return Conflict{Field: "sha256", Reason: "The uploaded file does not match the SHA-256 you gave."}
	}
	return nil
}

func (verifier Verifier) state(record Record, inspection blobs.Inspection) string {
	if (blobs.MediaTypes{}).Consistent(record.MediaType, inspection.SniffedType) {
		return States{}.Ready()
	}
	return States{}.Blocked()
}
