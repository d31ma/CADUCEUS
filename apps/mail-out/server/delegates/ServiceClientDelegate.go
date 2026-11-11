import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// @Delegate
// @NeedsAll()
type ServiceClientDelegate struct {
	settings *SettingsDelegate
}

func (ServiceClientDelegate) readLimit() int64 { return 1048576 }

func (client *ServiceClientDelegate) Get(address string, realm string, authorize func(string) (string, error), target any) (int, error) {
	authorization, tokenErr := authorize(realm)
	request, requestErr := http.NewRequest(http.MethodGet, address, nil)
	response, sendErr := client.send(request, authorization, errors.Join(tokenErr, requestErr))
	status, body, readErr := client.read(response, sendErr)
	return status, client.decode(status, body, target, readErr)
}

func (client *ServiceClientDelegate) Post(address string, realm string, authorize func(string) (string, error), payload any) (int, error) {
	authorization, tokenErr := authorize(realm)
	body, encodeErr := json.Marshal(payload)
	request, requestErr := http.NewRequest(http.MethodPost, address, bytes.NewReader(body))
	response, sendErr := client.send(request, authorization, errors.Join(tokenErr, encodeErr, requestErr))
	status, _, readErr := client.read(response, sendErr)
	return status, readErr
}

func (client *ServiceClientDelegate) send(request *http.Request, authorization string, failure error) (*http.Response, error) {
	if failure != nil {
		return nil, failure
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	return (&http.Client{Timeout: client.settings.Timeout()}).Do(request)
}

func (client *ServiceClientDelegate) read(response *http.Response, failure error) (int, []byte, error) {
	if failure != nil {
		return 0, nil, failure
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, client.readLimit()))
	return response.StatusCode, body, readErr
}

func (ServiceClientDelegate) decode(status int, body []byte, target any, failure error) error {
	if failure != nil || status != http.StatusOK {
		return failure
	}
	return json.Unmarshal(body, target)
}
