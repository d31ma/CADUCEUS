package blobs

import (
	"fmt"
	"net/http"
)

type BadKey struct {
	Part        string
	Value       string
	ErrorNumber int
}

func (BadKey) Domain(value string) BadKey {
	return BadKey{Part: "domain", Value: value, ErrorNumber: 21}
}

func (BadKey) Name(value string) BadKey {
	return BadKey{Part: "name", Value: value, ErrorNumber: 22}
}

func (bad BadKey) Error() string {
	return fmt.Sprintf("the blob key %s %q is not allowed", bad.Part, bad.Value)
}

func (BadKey) HttpStatus() int {
	return http.StatusNotFound
}

func (bad BadKey) Number() int {
	return bad.ErrorNumber
}
