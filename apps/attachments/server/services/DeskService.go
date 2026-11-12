import (
	"os"

	"mesail.dev/packages/attachments"
)

// @Service
type DeskService struct {
	// @Needs
	dataSource DataSource
}

func (service *DeskService) Desk() attachments.Desk {
	return attachments.DeskFactory{Environment: os.Getenv}.Desk(service.dataSource)
}
