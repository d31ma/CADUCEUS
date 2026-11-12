import (
	"errors"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

// @Service
type FailureService struct {
}

func (service *FailureService) Translated(failed error) error {
	return service.translated(failed)
}

func (service *FailureService) translated(failed error) error {
	invalid := attachments.Invalid{}
	conflict := attachments.Conflict{}
	large := blobs.TooLarge{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidUpload{Field: invalid.Field, Reason: invalid.Reason, ErrorNumber: invalid.ErrorNumber}
	case errors.As(failed, &conflict):
		return &AttachmentConflict{Field: conflict.Field, Reason: conflict.Reason, ErrorNumber: conflict.ErrorNumber}
	case errors.As(failed, &large):
		return &AttachmentTooLarge{Limit: large.Limit}
	case errors.Is(failed, attachments.Missing{}):
		return &AttachmentNotFound{}
	case errors.Is(failed, attachments.Forbidden{}):
		return &Forbidden{Message: "Only a service may read attachment details."}
	case errors.Is(failed, attachments.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
