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

// Message is the text to show for a failed request: the network message when
// the server was not reached, and the fallback when it refused.
func (HTTPUtils) Message(failure error, fallback string, network string) string {
	return HTTPUtils{}.pick(HTTPUtils{}.coded(failure, HTTPUtils{}.statusCode()), fallback, network)
}

// Missed is true when a request failed or answered without the field that
// proves it worked.
func (HTTPUtils) Missed(failure error, answered string) bool {
	return HTTPUtils{}.missed(failure, answered)
}

func (HTTPUtils) missed(failure error, answered string) bool {
	return failure != nil || answered == ""
}

func (HTTPUtils) coded(failure error, code string) bool {
	return failure != nil && strings.HasPrefix(failure.Error(), code)
}

func (HTTPUtils) pick(refused bool, fallback string, network string) string {
	if refused {
		return fallback
	}
	return network
}

func (HTTPUtils) statusCode() string { return "HTTP_STATUS" }
