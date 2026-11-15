package backups

import (
	"cmp"
	"errors"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

type ByteCheck struct{}

func (check ByteCheck) Check(store blobs.Store, pending Pending) error {
	reader, err := store.Read(BlobNames{}.Key(pending.Blob))
	if err != nil {
		return check.readFailure(err)
	}
	defer reader.Close()
	inspection, err := blobs.Inspector{}.Inspect(reader, pending.Size)
	if err != nil {
		return check.inspectFailure(err)
	}
	return cmp.Or(check.checkSize(pending, inspection), check.checkSha256(pending, inspection))
}

func (ByteCheck) readFailure(cause error) error {
	if errors.As(cause, &blobs.NotFound{}) {
		return attachments.Conflict{}.NotUploaded()
	}
	return attachments.Unavailable{Operation: "backup read"}
}

func (ByteCheck) inspectFailure(cause error) error {
	if errors.As(cause, &blobs.TooLarge{}) {
		return cause
	}
	return attachments.Unavailable{Operation: "backup inspect"}
}

func (ByteCheck) checkSize(pending Pending, inspection blobs.Inspection) error {
	if inspection.Size != pending.Size {
		return attachments.Conflict{}.SizeMismatch()
	}
	return nil
}

func (ByteCheck) checkSha256(pending Pending, inspection blobs.Inspection) error {
	if inspection.Sha256 != pending.Sha256 {
		return attachments.Conflict{}.Sha256Mismatch()
	}
	return nil
}
