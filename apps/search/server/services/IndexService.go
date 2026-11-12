import (
	"os"

	"mesail.dev/packages/search"
)

// @Service
type IndexService struct {
	// @Needs
	dataSource DataSource
}

func (service *IndexService) Index() search.Service {
	return search.ServiceFactory{Environment: os.Getenv}.Service(service.dataSource)
}
