import "net/http"

// @Service
type SearchService struct {
	// @Needs
	indexes *IndexService
	// @Needs
	failures *FailureService
}

func (service *SearchService) Search(request *http.Request) (SearchPage, error) {
	page := SearchPage{}
	failed := service.indexes.Index().SearchInto(request.Header.Get("Authorization"), request.URL.Query(), &page)
	return page, service.failures.Translated(failed)
}
