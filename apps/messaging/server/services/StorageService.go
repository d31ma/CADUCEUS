import (
	"time"

	"mesail.dev/packages/adapters"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/storage"
	"mesail.dev/packages/syncing"
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
	plain := storage.Storage{Source: service.dataSource, Scroll: service.scroll, Settings: settings, SettingsFailure: failure}
	plain.Pusher = syncing.NewChangeLog(service.pusherOrDefault(), plain.Stores(), time.Now)
	return plain
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
