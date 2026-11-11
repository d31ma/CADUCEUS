import (
	"errors"
	"os"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

// @RequestRoute("/api/v1/attachments")
// @SignedIn
type AttachmentsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute
// @ResponseStatus(201)
func (controller *AttachmentsController) PostAttachment(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateUploadRequest,
) (Upload, error) {
	terms := attachments.UploadRequest{FileName: request.FileName, MediaType: request.MediaType, Size: request.Size, Sha256: request.Sha256}
	upload, failed := controller.desk().Create(authorization, terms)
	return Upload{AttachmentId: upload.AttachmentId, UploadUrl: upload.UploadUrl, ExpiresAt: upload.ExpiresAt}, controller.failure(failed)
}

// @PostRoute("/{attachment_id}/complete")
func (controller *AttachmentsController) PostComplete(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) (Attachment, error) {
	view, failed := controller.desk().Complete(authorization, attachmentId)
	attachment := Attachment{AttachmentId: view.AttachmentId, FileName: view.FileName, MediaType: view.MediaType, Size: view.Size, Sha256: view.Sha256, State: view.State}
	return attachment, controller.failure(failed)
}

// @GetRoute("/{attachment_id}")
func (controller *AttachmentsController) GetAttachment(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) (Download, error) {
	download, failed := controller.desk().Download(authorization, attachmentId)
	return Download{Url: download.Url, ExpiresAt: download.ExpiresAt}, controller.failure(failed)
}

func (controller *AttachmentsController) desk() attachments.Desk {
	return attachments.DeskFactory{Environment: os.Getenv}.Desk(controller.dataSource)
}

func (controller *AttachmentsController) failure(failed error) error {
	invalid := attachments.Invalid{}
	conflict := attachments.Conflict{}
	large := blobs.TooLarge{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidUpload{Field: invalid.Field, Reason: invalid.Reason}
	case errors.As(failed, &conflict):
		return &AttachmentConflict{Field: conflict.Field, Reason: conflict.Reason}
	case errors.As(failed, &large):
		return &AttachmentTooLarge{Limit: large.Limit}
	case errors.Is(failed, attachments.Missing{}):
		return &AttachmentNotFound{}
	case errors.Is(failed, attachments.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
