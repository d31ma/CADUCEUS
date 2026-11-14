package attachments

import (
	"errors"

	"mesail.dev/packages/blobs"
)

type StoreFactory struct{}

func (factory StoreFactory) Store(settings Settings) (blobs.Store, error) {
	problems := errors.Join(factory.removedAdapter(settings), factory.removedBucket(settings), factory.missingSecret(settings))
	return factory.checked(settings, problems)
}

func (factory StoreFactory) checked(settings Settings, problems error) (blobs.Store, error) {
	if problems != nil {
		return nil, problems
	}
	return factory.local(settings), nil
}

func (factory StoreFactory) removedAdapter(settings Settings) error {
	if settings.Adapter == factory.localAdapter() {
		return nil
	}
	return RemovedSetting{Name: "adapter", Value: settings.Adapter}
}

func (StoreFactory) removedBucket(settings Settings) error {
	if settings.Bucket == "" {
		return nil
	}
	return RemovedSetting{Name: "bucket", Value: settings.Bucket}
}

func (StoreFactory) missingSecret(settings Settings) error {
	if settings.Link.Secret != "" {
		return nil
	}
	return Unavailable{Operation: "local link signing without a secret"}
}

func (StoreFactory) local(settings Settings) blobs.Store {
	return blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
}

func (StoreFactory) localAdapter() string {
	return "local"
}
