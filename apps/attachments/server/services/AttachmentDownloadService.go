import (
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

// @Service
type AttachmentDownloadService struct {
	// @Needs
	callerService *CallerService
	// @Needs
	settingsService *SettingsService
	// @Needs
	blobStoreService *BlobStoreService
	// @Needs
	attachmentRecordService *AttachmentRecordService
	// @Needs
	readerAccessService *ReaderAccessService
}

func (service *AttachmentDownloadService) Download(authorization string, attachmentId string) (Download, error) {
	caller, err := service.callerService.Caller(authorization)
	if err != nil {
		return Download{}, err
	}
	return service.download(caller, AccessRequest{AttachmentId: attachmentId, Authorization: authorization})
}

func (service *AttachmentDownloadService) download(caller oidc.Caller, request AccessRequest) (Download, error) {
	settings := service.settingsService.Settings()
	record, err := service.attachmentRecordService.Lookup(caller.DomainId, request.AttachmentId)
	if err != nil {
		return Download{}, err
	}
	request.Record = record
	request.Messaging = settings.Messaging
	err = service.readerAccessService.Check(caller, request)
	if err != nil {
		return Download{}, err
	}
	err = service.checkReady(record)
	if err != nil {
		return Download{}, err
	}
	return service.signed(settings, record)
}

func (service *AttachmentDownloadService) checkReady(record AttachmentRecord) error {
	if record.State != service.readyState() {
		return &AttachmentConflict{Field: "state", Reason: "The attachment is not ready to download."}
	}
	return nil
}

func (service *AttachmentDownloadService) signed(settings AttachmentSettings, record AttachmentRecord) (Download, error) {
	store, err := service.blobStoreService.Store(settings)
	if err != nil {
		return Download{}, err
	}
	expiresAt := service.settingsService.LinkExpiry(settings)
	terms := blobs.DownloadTerms{FileName: record.FileName, MediaType: record.MediaType}
	downloadUrl, err := store.SignDownload(service.attachmentRecordService.Key(record), terms, expiresAt)
	if err != nil {
		return Download{}, &StorageUnavailable{Operation: "sign download"}
	}
	return Download{Url: downloadUrl, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (service *AttachmentDownloadService) readyState() string {
	return "ready"
}
