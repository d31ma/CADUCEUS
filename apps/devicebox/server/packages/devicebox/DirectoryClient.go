package devicebox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type DirectoryClient struct {
	baseUrl string
	tokens  TokenSource
	http    *http.Client
}

func NewDirectoryClient(baseUrl string, tokens TokenSource, client *http.Client) DirectoryClient {
	return DirectoryClient{baseUrl: strings.TrimSuffix(baseUrl, "/"), tokens: tokens, http: client}
}

func (client DirectoryClient) Owners(domainId string, deviceIds []string) (map[string]string, error) {
	authorization, err := client.tokens.Authorization(domainId)
	if err != nil {
		return nil, Unavailable{}.DirectoryDown(err)
	}
	body, err := json.Marshal(deviceOwnersQuery{DomainId: domainId, DeviceIds: deviceIds})
	if err != nil {
		return nil, Unavailable{}.DirectoryDown(err)
	}
	request, err := http.NewRequest(http.MethodPost, client.baseUrl+client.path(), bytes.NewReader(body))
	if err != nil {
		return nil, Unavailable{}.DirectoryDown(err)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	return client.perform(request)
}

func (client DirectoryClient) perform(request *http.Request) (map[string]string, error) {
	response, err := client.http.Do(request)
	if err != nil {
		return nil, Unavailable{}.DirectoryDown(err)
	}
	defer response.Body.Close()
	err = client.statusRefusal(response.StatusCode)
	if err != nil {
		return nil, err
	}
	return client.decoded(response)
}

func (DirectoryClient) statusRefusal(status int) error {
	if status != http.StatusOK {
		return Unavailable{}.DirectoryDown(fmt.Errorf("the device list answered %d", status))
	}
	return nil
}

func (DirectoryClient) decoded(response *http.Response) (map[string]string, error) {
	answer := deviceOwners{}
	err := json.NewDecoder(response.Body).Decode(&answer)
	if err != nil || answer.Owners == nil {
		return nil, Unavailable{}.DirectoryUnreadable(err)
	}
	return answer.Owners, nil
}

func (DirectoryClient) path() string {
	return "/api/v1/directory/devices/owners"
}
