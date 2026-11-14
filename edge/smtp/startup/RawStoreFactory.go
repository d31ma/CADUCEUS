package startup

import (
	"mesail.dev/edge/smtp/adapters"
	"mesail.dev/edge/smtp/reception"
	"mesail.dev/packages/blobs"
)

type RawStoreFactory struct{}

func (RawStoreFactory) Store(settings ListenerSettings) reception.RawMessageStore {
	return adapters.NewBlobRawStore(blobs.NewLocalFolder(settings.RawStoreFolder, "", blobs.NewLocalSigner(nil)))
}
