package channels

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type DirectoryClient struct {
	baseUrl string
	http    *http.Client
}

func NewDirectoryClient(baseUrl string, client *http.Client) DirectoryClient {
	return DirectoryClient{baseUrl: strings.TrimSuffix(baseUrl, "/"), http: client}
}

func (client DirectoryClient) MemberInDomain(authorization string, memberId string) (bool, error) {
	status, err := client.exchange(http.MethodGet, client.memberAddress(memberId), authorization, nil)
	if err != nil {
		return false, err
	}
	return client.memberAnswer(status)
}

func (client DirectoryClient) ClaimChannelAddress(authorization string, address string, channelId string) error {
	body, _ := json.Marshal(ChannelAddressRequest{Address: address, ChannelId: channelId})
	status, err := client.exchange(http.MethodPost, client.channelAddresses(), authorization, body)
	if err != nil {
		return err
	}
	return client.claimAnswer(status, address)
}

func (client DirectoryClient) ReleaseChannelAddress(authorization string, address string) error {
	status, err := client.exchange(http.MethodDelete, client.channelAddress(address), authorization, nil)
	if err != nil {
		return err
	}
	return client.releaseAnswer(status)
}

func (client DirectoryClient) exchange(method string, address string, authorization string, body []byte) (int, error) {
	request, err := http.NewRequest(method, address, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	return client.perform(request)
}

func (client DirectoryClient) perform(request *http.Request) (int, error) {
	response, err := client.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	return response.StatusCode, nil
}

func (client DirectoryClient) memberAnswer(status int) (bool, error) {
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, DirectoryFailed{Action: "the member lookup", Status: status}
	}
}

func (client DirectoryClient) claimAnswer(status int, address string) error {
	switch status {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusConflict:
		return AddressTaken{Address: address}
	case http.StatusBadRequest:
		return AddressRefused{Address: address}
	default:
		return DirectoryFailed{Action: "the channel address", Status: status}
	}
}

func (client DirectoryClient) releaseAnswer(status int) error {
	switch status {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return DirectoryFailed{Action: "the channel address release", Status: status}
	}
}

func (client DirectoryClient) memberAddress(memberId string) string {
	return client.baseUrl + "/api/v1/directory/members/" + url.PathEscape(memberId)
}

func (client DirectoryClient) channelAddresses() string {
	return client.baseUrl + "/api/v1/directory/channel-addresses"
}

func (client DirectoryClient) channelAddress(address string) string {
	return client.channelAddresses() + "/" + url.PathEscape(address)
}
