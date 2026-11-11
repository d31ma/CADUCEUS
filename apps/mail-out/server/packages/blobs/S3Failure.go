package blobs

import (
	"fmt"
	"net/http"
)

type S3Failure struct {
	Method string
	Status int
}

func (failure S3Failure) Error() string {
	return fmt.Sprintf("the S3 %s request answered HTTP %d", failure.Method, failure.Status)
}

func (S3Failure) HttpStatus() int {
	return http.StatusBadGateway
}
