package startup

import (
	"net/http"
	"os"
	"strings"
	"time"

	"mesail.dev/edge/smtp/adapters"
	"mesail.dev/edge/smtp/reception"
	"mesail.dev/packages/blobs"
)

type RawStoreFactory struct{}

func (factory RawStoreFactory) Store(settings ListenerSettings) (reception.RawMessageStore, error) {
	store, err := factory.chosen(settings)(settings)
	if err != nil {
		return nil, err
	}
	return adapters.NewBlobRawStore(store), nil
}

func (factory RawStoreFactory) chosen(settings ListenerSettings) func(ListenerSettings) (blobs.Store, error) {
	if settings.RawStoreS3.Bucket == "" {
		return factory.localFolder
	}
	return factory.s3
}

func (RawStoreFactory) localFolder(settings ListenerSettings) (blobs.Store, error) {
	return blobs.NewLocalFolder(settings.RawStoreFolder, "", blobs.NewLocalSigner(nil)), nil
}

func (factory RawStoreFactory) s3(settings ListenerSettings) (blobs.Store, error) {
	secret, err := factory.secret(settings.RawStoreS3.SecretFile)
	if err != nil {
		return nil, err
	}
	return blobs.NewS3Store(factory.s3Settings(settings.RawStoreS3, secret), factory.uploadClient(settings.RawStoreS3), time.Now), nil
}

func (factory RawStoreFactory) uploadClient(settings S3RawStoreSettings) *http.Client {
	return &http.Client{Timeout: factory.timeoutOrDefault(settings.Timeout)}
}

func (factory RawStoreFactory) timeoutOrDefault(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return factory.defaultUploadTimeout()
	}
	return timeout
}

func (RawStoreFactory) defaultUploadTimeout() time.Duration {
	return 5 * time.Minute
}

func (RawStoreFactory) s3Settings(settings S3RawStoreSettings, secret string) blobs.S3Settings {
	return blobs.S3Settings{
		Endpoint:        settings.Endpoint,
		Region:          settings.Region,
		Bucket:          settings.Bucket,
		AccessKeyId:     settings.AccessKeyId,
		SecretAccessKey: secret,
	}
}

func (factory RawStoreFactory) secret(file string) (string, error) {
	content, err := os.ReadFile(file)
	return factory.nonEmpty(strings.TrimSpace(string(content)), err)
}

func (RawStoreFactory) nonEmpty(secret string, readError error) (string, error) {
	if readError != nil || secret == "" {
		return "", InvalidSetting{Name: S3RawStoreNames{}.SecretFile(), Reason: "the file must exist and hold the secret access key"}
	}
	return secret, nil
}
