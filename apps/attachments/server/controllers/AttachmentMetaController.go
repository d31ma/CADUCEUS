import (
	"errors"
	"os"

	"mesail.dev/packages/attachments"
)

// @RequestRoute("/api/v1/attachments")
// @Allow(Service)
type AttachmentMetaController struct {
	// @Needs
	dataSource DataSource
}

// @GetRoute("/{attachment_id}/meta")
func (controller *AttachmentMetaController) GetMeta(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) (Attachment, error) {
	view, failed := controller.desk().Meta(authorization, attachmentId)
	attachment := Attachment{AttachmentId: view.AttachmentId, FileName: view.FileName, MediaType: view.MediaType, Size: view.Size, Sha256: view.Sha256, State: view.State}
	return attachment, controller.failure(failed)
}

func (controller *AttachmentMetaController) desk() attachments.Desk {
	return attachments.DeskFactory{Environment: os.Getenv}.Desk(controller.dataSource)
}

func (controller *AttachmentMetaController) failure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.Is(failed, attachments.Missing{}):
		return &AttachmentNotFound{}
	case errors.Is(failed, attachments.Forbidden{}):
		return &Forbidden{Message: "Only a service may read attachment details."}
	case errors.Is(failed, attachments.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
