package search

type Reader interface {
	Readable(authorization string) (map[string]bool, error)
}
