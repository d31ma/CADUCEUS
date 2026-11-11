package blobs

import (
	"fmt"
	"net/http"
)

type NotFound struct {
	Path string
}

func (missing NotFound) Error() string {
	return fmt.Sprintf("no blob is stored at %q", missing.Path)
}

func (NotFound) HttpStatus() int {
	return http.StatusNotFound
}
