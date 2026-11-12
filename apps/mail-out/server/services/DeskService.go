import (
	"os"

	"mesail.dev/packages/mailout"
)

// @Service
type DeskService struct {
	// @Needs
	dataSource DataSource
}

func (service *DeskService) Desk() mailout.Desk {
	return mailout.DeskFactory{Environment: os.Getenv}.Desk(service.dataSource)
}
