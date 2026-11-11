package mailin

import (
	"net/http"
	"time"

	"mesail.dev/packages/blobs"
)

type BlobStoreFactory struct {
	Environment func(string) string
}

func (factory BlobStoreFactory) Store(settings BlobSettings) blobs.Store {
	if factory.incompleteLocal(settings) {
		return MissingBlobs{}
	}
	return factory.chosen(settings)
}

func (factory BlobStoreFactory) incompleteLocal(settings BlobSettings) bool {
	local := settings.Adapter == SettingsReader{}.LocalAdapter()
	return local && (settings.Folder == "" || settings.Link.Secret == "")
}

func (factory BlobStoreFactory) chosen(settings BlobSettings) blobs.Store {
	if settings.Adapter == (SettingsReader{}).LocalAdapter() {
		return factory.local(settings)
	}
	return factory.s3(settings)
}

func (BlobStoreFactory) local(settings BlobSettings) blobs.Store {
	return blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
}

func (factory BlobStoreFactory) s3(settings BlobSettings) blobs.Store {
	s3Settings := blobs.S3Settings{
		Endpoint: settings.Endpoint, PublicEndpoint: settings.Public, Region: settings.Region, Bucket: settings.Bucket,
		AccessKeyId: factory.Environment("AWS_ACCESS_KEY_ID"), SecretAccessKey: factory.Environment("AWS_SECRET_ACCESS_KEY"), SessionToken: factory.Environment("AWS_SESSION_TOKEN"),
	}
	return blobs.NewS3Store(s3Settings, http.DefaultClient, time.Now)
}
