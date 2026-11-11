import (
	"mesail.dev/packages/adapters"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/storage"
)

// @Service
type StorageService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	settings *SettingsService
	pusher   outbox.Pusher
}

func (service *StorageService) Storage() storage.Storage {
	settings, failure := service.settings.Outbox()
	return storage.Storage{Source: service.dataSource, Scroll: service.scroll, Pusher: service.pusherOrDefault(), Settings: settings, SettingsFailure: failure}
}

func (service *StorageService) scroll(collection string, where map[string]any, cursor string, size int) (storage.Window, error) {
	window, failure := service.dataSource.Scroll(collection, where, cursor, size)
	return storage.Window{Content: window.Content, NextCursor: window.NextCursor}, failure
}

func (service *StorageService) pusherOrDefault() outbox.Pusher {
	if service.pusher != nil {
		return service.pusher
	}
	return adapters.NewDomainPusher(oidc.ServiceTokensFactory{}.Configured(), outbox.HTTPPusher{}.DefaultTimeout())
}
