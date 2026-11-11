import (
	"cmp"
	"errors"

	"mesail.dev/packages/blobs"
)

// @Service
type UploadVerifyService struct {
}

func (service *UploadVerifyService) Verify(store blobs.Store, key blobs.Key, record AttachmentRecord) (string, error) {
	reader, err := store.Read(key)
	if err != nil {
		return "", service.readFailure(err)
	}
	defer reader.Close()
	inspection, err := blobs.Inspector{}.Inspect(reader, record.Size)
	if err != nil {
		return "", service.inspectFailure(err)
	}
	err = cmp.Or(service.checkSize(record, inspection), service.checkSha256(record, inspection))
	if err != nil {
		return "", err
	}
	return service.state(record, inspection), nil
}

func (service *UploadVerifyService) readFailure(cause error) error {
	if errors.As(cause, &blobs.NotFound{}) {
		return &AttachmentConflict{Field: "file", Reason: "The file has not been uploaded yet."}
	}
	return &StorageUnavailable{Operation: "read"}
}

func (service *UploadVerifyService) inspectFailure(cause error) error {
	large := blobs.TooLarge{}
	if errors.As(cause, &large) {
		return &AttachmentTooLarge{Limit: large.Limit}
	}
	return &StorageUnavailable{Operation: "inspect"}
}

func (service *UploadVerifyService) checkSize(record AttachmentRecord, inspection blobs.Inspection) error {
	if inspection.Size != record.Size {
		return &AttachmentConflict{Field: "size", Reason: "The uploaded file is smaller than the size you gave."}
	}
	return nil
}

func (service *UploadVerifyService) checkSha256(record AttachmentRecord, inspection blobs.Inspection) error {
	if inspection.Sha256 != record.Sha256 {
		return &AttachmentConflict{Field: "sha256", Reason: "The uploaded file does not match the SHA-256 you gave."}
	}
	return nil
}

func (service *UploadVerifyService) state(record AttachmentRecord, inspection blobs.Inspection) string {
	if (blobs.MediaTypes{}).Consistent(record.MediaType, inspection.SniffedType) {
		return "ready"
	}
	return "blocked"
}
