package mailin

import (
	"cmp"

	"mesail.dev/packages/blobs"
)

type BlobStoreFactory struct{}

func (factory BlobStoreFactory) Store(settings BlobSettings) blobs.Store {
	problem := cmp.Or(factory.removedAdapter(settings), factory.removedBucket(settings), factory.incompleteLocal(settings))
	return factory.checked(settings, problem)
}

func (factory BlobStoreFactory) checked(settings BlobSettings, problem string) blobs.Store {
	if problem != "" {
		return MissingBlobs{Problem: problem}
	}
	return factory.local(settings)
}

func (factory BlobStoreFactory) removedAdapter(settings BlobSettings) string {
	if settings.Adapter == (SettingsReader{}).LocalAdapter() {
		return ""
	}
	return factory.removed("adapter", settings.Adapter)
}

func (factory BlobStoreFactory) removedBucket(settings BlobSettings) string {
	if settings.Bucket == "" {
		return ""
	}
	return factory.removed("bucket", settings.Bucket)
}

func (BlobStoreFactory) incompleteLocal(settings BlobSettings) string {
	if settings.Folder != "" && settings.Link.Secret != "" {
		return ""
	}
	return MissingBlobs{}.Reason()
}

func (BlobStoreFactory) removed(name string, value string) string {
	return "the mail-in setting blobs." + name + " \"" + value + "\" was removed with the cloud option (ADR 0007): keep blobs in a local folder with the adapter \"local\""
}

func (BlobStoreFactory) local(settings BlobSettings) blobs.Store {
	return blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
}
