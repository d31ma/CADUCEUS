// StatusMarker is a field that marks an error answer with its status, such as
// "notFound" for 404.
type StatusMarker struct {
	Field  string
	Status int
}
