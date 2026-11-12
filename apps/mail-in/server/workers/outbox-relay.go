import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"mesail.dev/packages/mailin"
	"mesail.dev/packages/transport"
)

// @Schedule({"fixedDelaySeconds":60,"initialDelaySeconds":30})
type Handler struct{}

func (handler Handler) POST() {
	var snapshot struct {
		Yon struct {
			Server struct {
				Port int `json:"port"`
			} `json:"server"`
		} `json:"yon"`
	}
	json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &snapshot)
	authorization, failure := mailin.SettingsReader{Environment: os.Getenv}.Authorize()(mailin.Directory{}.InstallWide())
	address := "http://127.0.0.1:" + strconv.Itoa(snapshot.Yon.Server.Port) + "/api/v1/mail-in/outbox/relay"
	request, requestFailure := http.NewRequest(http.MethodPost, address, nil)
	handler.send(request, authorization, errors.Join(failure, requestFailure))
}

func (handler Handler) send(request *http.Request, authorization string, failure error) {
	if failure != nil {
		return
	}
	request.Header.Set("Authorization", authorization)
	response, sendFailure := transport.SingleUseTransport{}.Client(handler.timeout()).Do(request)
	handler.close(response, sendFailure)
}

func (Handler) close(response *http.Response, failure error) {
	if failure == nil {
		response.Body.Close()
	}
}

func (Handler) timeout() time.Duration {
	return 60 * time.Second
}
