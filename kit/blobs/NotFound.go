package blobs

import "fmt"

type NotFound struct {
	Path string
}

func (missing NotFound) Error() string {
	return fmt.Sprintf("no blob is stored at %q", missing.Path)
}
