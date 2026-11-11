package blobs

import (
	"fmt"
	"net/http"
)

type BadKey struct {
	Part  string
	Value string
}

func (bad BadKey) Error() string {
	return fmt.Sprintf("the blob key %s %q is not allowed", bad.Part, bad.Value)
}

func (BadKey) HttpStatus() int {
	return http.StatusNotFound
}
