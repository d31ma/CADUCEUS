package blobs

import (
	"mime"
	"slices"
	"strings"
)

type MediaTypes struct{}

func (types MediaTypes) Valid(mediaType string) bool {
	return types.wellFormed(mime.ParseMediaType(mediaType))
}

func (types MediaTypes) Consistent(declared string, sniffed string) bool {
	return types.allowed(types.base(declared), types.base(sniffed))
}

func (MediaTypes) wellFormed(base string, parameters map[string]string, err error) bool {
	return err == nil && strings.Count(base, "/") == 1
}

func (types MediaTypes) allowed(declared string, sniffed string) bool {
	return declared == sniffed || slices.Contains(types.vagueSniffs(), sniffed) || (types.family(declared) != types.imageFamily() && sniffed != types.html())
}

func (MediaTypes) base(mediaType string) string {
	base, _, _ := mime.ParseMediaType(mediaType)
	return base
}

func (MediaTypes) family(base string) string {
	family, _, _ := strings.Cut(base, "/")
	return family
}

func (MediaTypes) vagueSniffs() []string {
	return []string{"application/octet-stream", "text/plain", "text/xml"}
}

func (MediaTypes) imageFamily() string {
	return "image"
}

func (MediaTypes) html() string {
	return "text/html"
}
