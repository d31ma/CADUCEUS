package blobs

import "net/http"

type Expired struct{}

func (Expired) Error() string {
	return "the blob link has expired"
}

func (Expired) HttpStatus() int {
	return http.StatusForbidden
}
