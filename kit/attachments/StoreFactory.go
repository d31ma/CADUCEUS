package attachments

import (
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/transport"
)

type StoreFactory struct {
	Environment func(string) string
}

func (factory StoreFactory) Store(settings Settings) (blobs.Store, error) {
	err := factory.checkSecret(settings)
	if err != nil {
		return nil, err
	}
	return factory.chosen(settings), nil
}

func (factory StoreFactory) checkSecret(settings Settings) error {
	if settings.Adapter == factory.localAdapter() && settings.Link.Secret == "" {
		return Unavailable{Operation: "local link signing without a secret"}
	}
	return nil
}

func (factory StoreFactory) chosen(settings Settings) blobs.Store {
	if settings.Adapter == factory.localAdapter() {
		return factory.local(settings)
	}
	return factory.s3(settings)
}

func (StoreFactory) local(settings Settings) blobs.Store {
	return blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
}

func (factory StoreFactory) s3(settings Settings) blobs.Store {
	s3Settings := blobs.S3Settings{
		Endpoint: settings.Endpoint, PublicEndpoint: settings.Public, Region: settings.Region, Bucket: settings.Bucket,
		AccessKeyId: factory.Environment("AWS_ACCESS_KEY_ID"), SecretAccessKey: factory.Environment("AWS_SECRET_ACCESS_KEY"), SessionToken: factory.Environment("AWS_SESSION_TOKEN"),
	}
	return blobs.NewS3Store(s3Settings, transport.SingleUseTransport{}.Client(0), time.Now)
}

func (StoreFactory) localAdapter() string {
	return "local"
}
