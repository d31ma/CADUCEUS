import "mesail.dev/packages/oidc"

// @Service
type AttachmentCompleteService struct {
	// @Needs
	callerService *CallerService
	// @Needs
	settingsService *SettingsService
	// @Needs
	blobStoreService *BlobStoreService
	// @Needs
	attachmentRecordService *AttachmentRecordService
	// @Needs
	uploadVerifyService *UploadVerifyService
}

func (service *AttachmentCompleteService) Complete(authorization string, attachmentId string) (Attachment, error) {
	caller, err := service.callerService.Caller(authorization)
	if err != nil {
		return Attachment{}, err
	}
	return service.complete(caller, attachmentId)
}

func (service *AttachmentCompleteService) complete(caller oidc.Caller, attachmentId string) (Attachment, error) {
	record, err := service.attachmentRecordService.Lookup(caller.DomainId, attachmentId)
	if err != nil {
		return Attachment{}, err
	}
	err = service.checkUploader(caller, record)
	if err != nil {
		return Attachment{}, err
	}
	return service.completed(record)
}

func (service *AttachmentCompleteService) checkUploader(caller oidc.Caller, record AttachmentRecord) error {
	if caller.MemberId != record.UploaderId {
		return &AttachmentNotFound{}
	}
	return nil
}

func (service *AttachmentCompleteService) completed(record AttachmentRecord) (Attachment, error) {
	if record.State != "pending" {
		return service.attachmentRecordService.View(record), nil
	}
	return service.verified(record)
}

func (service *AttachmentCompleteService) verified(record AttachmentRecord) (Attachment, error) {
	store, err := service.blobStoreService.Store(service.settingsService.Settings())
	if err != nil {
		return Attachment{}, err
	}
	state, err := service.uploadVerifyService.Verify(store, service.attachmentRecordService.Key(record), record)
	if err != nil {
		return Attachment{}, err
	}
	saved, err := service.attachmentRecordService.SetState(record, state)
	return service.attachmentRecordService.View(saved), err
}
