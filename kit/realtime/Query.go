package realtime

type Query struct {
	Collection string
	Where      map[string]any
	Cursor     string
	Size       int
}
