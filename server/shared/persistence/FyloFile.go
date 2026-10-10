// FyloFile describes one staged file to store in a FYLO file collection:
// its absolute path, its bucket key, its byte limit (nil for none) and the
// metadata stored beside it.
type FyloFile struct {
	Path     string
	Key      string
	MaxBytes *int
	Meta     map[string]any
}
