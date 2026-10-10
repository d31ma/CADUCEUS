import "strings"

// HTTPUtils reads the failures of the browser transport. The transport
// reports every error status as HTTP_STATUS and every unreachable server as
// HTTP_NETWORK; it hands over neither the status code nor the body, so a
// service shows its own fixed message for a refused request.
type HTTPUtils struct{}

// Refused is true when the server answered with an error status.
func (HTTPUtils) Refused(failure error) bool {
	return HTTPUtils{}.coded(failure, HTTPUtils{}.statusCode())
}

func (HTTPUtils) coded(failure error, code string) bool {
	return failure != nil && strings.HasPrefix(failure.Error(), code)
}

func (HTTPUtils) statusCode() string { return "HTTP_STATUS" }
