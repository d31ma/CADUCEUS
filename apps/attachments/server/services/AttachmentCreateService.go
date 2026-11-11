import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

// @Service
type AttachmentCreateService struct {
	// @Needs
	callerService *CallerService
	// @Needs
	settingsService *SettingsService
	// @Needs
	uploadCheckService *UploadCheckService
	// @Needs
	blobStoreService *BlobStoreService
	// @Needs
	attachmentRecordService *AttachmentRecordService
}

func (service *AttachmentCreateService) Create(authorization string, request CreateUploadRequest) (Upload, error) {
	caller, err := service.callerService.Caller(authorization)
	if err != nil {
		return Upload{}, err
	}
	return service.create(caller, request)
}

func (service *AttachmentCreateService) create(caller oidc.Caller, request CreateUploadRequest) (Upload, error) {
	settings := service.settingsService.Settings()
	err := service.uploadCheckService.Check(request, settings.Limit.Bytes)
	if err != nil {
		return Upload{}, err
	}
	store, err := service.blobStoreService.Store(settings)
	if err != nil {
		return Upload{}, err
	}
	collection, err := service.attachmentRecordService.Collection(caller.DomainId)
	if err != nil {
		return Upload{}, err
	}
	record, err := service.attachmentRecordService.Insert(collection, service.pending(caller, request))
	if err != nil {
		return Upload{}, err
	}
	return service.upload(store, record, service.settingsService.LinkExpiry(settings))
}

func (service *AttachmentCreateService) pending(caller oidc.Caller, request CreateUploadRequest) AttachmentRecord {
	key := blobs.Key{DomainId: caller.DomainId, Name: rand.Text()}
	return AttachmentRecord{
		DomainId: caller.DomainId, UploaderId: caller.MemberId, FileName: request.FileName, MediaType: request.MediaType,
		Size: request.Size, Sha256: request.Sha256, State: "pending", BlobKey: key.Path(),
	}
}

func (service *AttachmentCreateService) upload(store blobs.Store, record AttachmentRecord, expiresAt time.Time) (Upload, error) {
	terms := blobs.UploadTerms{MediaType: record.MediaType, Size: record.Size}
	uploadUrl, err := store.SignUpload(service.attachmentRecordService.Key(record), terms, expiresAt)
	if err != nil {
		return Upload{}, &StorageUnavailable{Operation: "sign upload"}
	}
	return Upload{AttachmentId: service.attachmentRecordService.WireId(record), UploadUrl: uploadUrl, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}
