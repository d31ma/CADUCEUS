package blobs

import "net/http"

type BadSignature struct{}

func (BadSignature) Error() string {
	return "the blob link signature is not valid"
}

func (BadSignature) HttpStatus() int {
	return http.StatusForbidden
}
