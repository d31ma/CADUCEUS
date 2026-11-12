import (
	"errors"
	"net/http"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

// @Schedule({"fixedDelaySeconds":30})
type Handler struct{}

func (handler Handler) POST() {
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(oidc.InstallWide{}.DomainId())
	handler.post(handler.base()+"/jobs/run", authorization, failure)
	handler.post(handler.base()+"/outbox/relay", authorization, failure)
}

func (handler Handler) post(address string, authorization string, failure error) {
	request, requestFailure := http.NewRequest(http.MethodPost, address, nil)
	handler.send(request, authorization, errors.Join(failure, requestFailure))
}

func (handler Handler) send(request *http.Request, authorization string, failure error) {
	if failure != nil {
		return
	}
	request.Header.Set("Authorization", authorization)
	response, sendFailure := transport.SingleUseTransport{}.Client(handler.timeout()).Do(request)
	handler.closeBody(response, sendFailure)
}

func (Handler) closeBody(response *http.Response, failure error) {
	if failure == nil {
		response.Body.Close()
	}
}

func (Handler) base() string {
	return "http://127.0.0.1:8107/api/v1/notify"
}

func (Handler) timeout() time.Duration {
	return 60 * time.Second
}
