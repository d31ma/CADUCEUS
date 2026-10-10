// @HTTPClient
type CategoriesClient interface {
	// @GetRequest("/categories")
	Catalog(
		// @RequestHeader("Authorization")
		authorization string,
	) (CategoryCatalog, error)

	// @PostRequest("/categories")
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		category BehaviorValue,
	) (CategoryCreated, error)
}
