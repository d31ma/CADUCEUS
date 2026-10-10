// @HTTPClient
type CategoriesClient interface {
	// @GetRequest("/categories")
	Catalog(
		// @RequestHeader("Authorization")
		authorization string,
	) (CategoryCatalog, error)

	// @PostRequest("/categories")
	Create(
		// @RequestBody
		category BehaviorValue,
		// @RequestHeader("Authorization")
		authorization string,
	) (CreatedCategory, error)
}
