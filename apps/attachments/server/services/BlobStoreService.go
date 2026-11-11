import (
	"net/http"
	"os"
	"time"

	"mesail.dev/packages/blobs"
)

// @Service
type BlobStoreService struct {
}

func (service *BlobStoreService) Store(settings AttachmentSettings) (blobs.Store, error) {
	err := service.checkSecret(settings)
	if err != nil {
		return nil, err
	}
	return service.chosen(settings), nil
}

func (service *BlobStoreService) checkSecret(settings AttachmentSettings) error {
	if settings.Adapter == service.localAdapter() && settings.Link.Secret == "" {
		return &StorageUnavailable{Operation: "local link signing without a secret"}
	}
	return nil
}

func (service *BlobStoreService) chosen(settings AttachmentSettings) blobs.Store {
	if settings.Adapter == service.localAdapter() {
		return service.local(settings)
	}
	return service.s3(settings)
}

func (service *BlobStoreService) local(settings AttachmentSettings) blobs.Store {
	return blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
}

func (service *BlobStoreService) s3(settings AttachmentSettings) blobs.Store {
	s3Settings := blobs.S3Settings{
		Endpoint: settings.Endpoint, PublicEndpoint: settings.Public, Region: settings.Region, Bucket: settings.Bucket,
		AccessKeyId: os.Getenv("AWS_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"), SessionToken: os.Getenv("AWS_SESSION_TOKEN"),
	}
	return blobs.NewS3Store(s3Settings, http.DefaultClient, time.Now)
}

func (service *BlobStoreService) localAdapter() string {
	return "local"
}
