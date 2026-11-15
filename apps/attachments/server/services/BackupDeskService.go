import (
	"encoding/json"
	"os"

	"mesail.dev/packages/backups"
)

// @Service
type BackupDeskService struct {
	// @Needs
	dataSource DataSource
}

func (service *BackupDeskService) Desk() backups.Desk {
	return backups.DeskFactory{Environment: os.Getenv}.Desk(service.dataSource, service.scroller())
}

func (service *BackupDeskService) EventDesk() backups.EventDesk {
	return backups.DeskFactory{Environment: os.Getenv}.EventDesk(service.dataSource, service.scroller())
}

func (service *BackupDeskService) scroller() backups.Scroller {
	return func(collection string, where map[string]any, cursor string, size int) (backups.Window, error) {
		window, failure := service.dataSource.Scroll(collection, where, cursor, size)
		return backups.Window{Content: json.RawMessage(window.Content), NextCursor: window.NextCursor}, failure
	}
}
