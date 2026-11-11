import (
	"errors"
	"net/http"
	"os"

	"mesail.dev/packages/search"
)

// @RequestRoute("/api/v1/search")
// @SignedIn
type SearchController struct {
	// @Needs
	dataSource DataSource
}

// @GetRoute
func (controller *SearchController) GetSearch(request *http.Request) (SearchPage, error) {
	page := SearchPage{}
	service := search.ServiceFactory{Environment: os.Getenv}.Service(controller.dataSource)
	failure := service.SearchInto(request.Header.Get("Authorization"), request.URL.Query(), &page)
	return page, controller.failure(failure)
}

func (controller *SearchController) failure(failed error) error {
	invalid := search.InvalidRequest{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Message: invalid.Message, Field: invalid.Field}
	case errors.As(failed, &search.Unauthenticated{}):
		return &Unauthenticated{Message: failed.Error()}
	case errors.As(failed, &search.Forbidden{}):
		return &Forbidden{Message: failed.Error()}
	}
	return &Unavailable{Message: failed.Error()}
}
