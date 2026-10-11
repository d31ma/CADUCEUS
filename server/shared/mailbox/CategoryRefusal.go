// CategoryRefusal is why a category could not be created: a missing owner,
// a domain the owner is not assigned, or a bad request.
type CategoryRefusal struct {
	NotFound  bool
	Forbidden bool
	Message   string
}
