import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

// @Schedule({"fixedDelaySeconds":30,"initialDelaySeconds":10})
type Handler struct{}

func (Handler) timeout() time.Duration { return 60 * time.Second }

func (handler Handler) POST() {
	var snapshot struct {
		Yon struct {
			Server struct {
				Port int `json:"port"`
			} `json:"server"`
		} `json:"yon"`
	}
	json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &snapshot)
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(oidc.InstallWide{}.DomainId())
	base := "http://127.0.0.1:" + strconv.Itoa(snapshot.Yon.Server.Port) + "/api/v1/mail-out"
	handler.post(base+"/jobs/deliver", authorization, failure)
	handler.post(base+"/outbox/relay", authorization, failure)
}

func (handler Handler) post(address string, authorization string, failure error) {
	request, requestErr := http.NewRequest(http.MethodPost, address, nil)
	handler.send(request, authorization, errors.Join(failure, requestErr))
}

func (handler Handler) send(request *http.Request, authorization string, failure error) {
	if failure != nil {
		return
	}
	request.Header.Set("Authorization", authorization)
	response, sendErr := transport.SingleUseTransport{}.Client(handler.timeout()).Do(request)
	handler.close(response, sendErr)
}

func (Handler) close(response *http.Response, failure error) {
	if failure == nil {
		response.Body.Close()
	}
}
