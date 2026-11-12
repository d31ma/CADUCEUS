import "net/http"

// @RequestRoute("/api/v1/search")
// @SignedIn
type SearchController struct {
	// @Needs
	searches *SearchService
}

// @GetRoute
func (controller *SearchController) GetSearch(request *http.Request) (SearchPage, error) {
	return controller.searches.Search(request)
}
