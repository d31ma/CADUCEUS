package mailout

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type ServiceClient struct {
	Client    *http.Client
	Authorize func(realm string) (string, error)
}

func (ServiceClient) readLimit() int64 { return 1048576 }

func (client ServiceClient) Get(address string, realm string, target any) (int, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return 0, err
	}
	status, body, err := client.send(request, realm)
	if err != nil {
		return status, err
	}
	return client.decode(status, body, target)
}

func (client ServiceClient) Post(address string, realm string, payload any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequest(http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	status, _, err := client.send(request, realm)
	return status, err
}

func (client ServiceClient) send(request *http.Request, realm string) (int, []byte, error) {
	authorization, err := client.Authorize(realm)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, client.readLimit()))
	return response.StatusCode, body, err
}

func (ServiceClient) decode(status int, body []byte, target any) (int, error) {
	if status != http.StatusOK {
		return status, nil
	}
	return status, json.Unmarshal(body, target)
}
