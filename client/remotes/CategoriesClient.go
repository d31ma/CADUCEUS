// @HTTPClient("/categories")
type CategoriesClient interface {
	// @GetRequest
	Catalog(
		// @RequestHeader("Authorization")
		authorization string,
	) (CategoryCatalog, error)
}
