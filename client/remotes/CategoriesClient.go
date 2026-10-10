// @HTTPClient
type CategoriesClient interface {
	// @GetRequest("/categories")
	Catalog(
		// @RequestHeader("Authorization")
		authorization string,
	) (CategoryCatalog, error)
}
