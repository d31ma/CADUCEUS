// @HTTPClient("/categories")
type CategoriesClient interface {
	// @GetRequest
	Catalog(
		// @RequestHeader("Authorization")
		authorization string,
	) (CategoryCatalog, error)

	// @PostRequest
	Create(
		// @RequestHeader("Authorization")
		authorization string,
		// @RequestBody
		category BehaviorValue,
	) (CategoryCreated, error)
}
