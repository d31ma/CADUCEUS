package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"mesail.dev/edge/smtp/reception"
)

type HttpDirectory struct {
	baseUrl  string
	requests AuthorizedRequests
	client   *http.Client
}

func NewHttpDirectory(baseUrl string, tokens reception.ServiceTokens, client *http.Client) HttpDirectory {
	return HttpDirectory{baseUrl: strings.TrimSuffix(baseUrl, "/"), requests: NewAuthorizedRequests(tokens), client: client}
}

func (directory HttpDirectory) Lookup(ctx context.Context, address string) (reception.Address, error) {
	request, err := directory.requests.New(ctx, http.MethodGet, directory.addressUrl(address), nil)
	if err != nil {
		return reception.Address{}, err
	}
	response, err := directory.client.Do(request)
	if err != nil {
		return reception.Address{}, err
	}
	defer directory.requests.Finish(response)
	return directory.readerFor(response.StatusCode)(response)
}

func (directory HttpDirectory) addressUrl(address string) string {
	return directory.baseUrl + "/api/v1/directory/addresses/" + url.PathEscape(address)
}

func (directory HttpDirectory) readerFor(status int) func(*http.Response) (reception.Address, error) {
	switch status {
	case http.StatusOK:
		return directory.found
	case http.StatusNotFound:
		return directory.notFound
	default:
		return directory.unexpected
	}
}

func (HttpDirectory) found(response *http.Response) (reception.Address, error) {
	var address reception.Address
	err := json.NewDecoder(response.Body).Decode(&address)
	return address, err
}

func (directory HttpDirectory) notFound(response *http.Response) (reception.Address, error) {
	var answer ApiError
	json.NewDecoder(response.Body).Decode(&answer)
	return reception.Address{}, directory.notFoundError(answer.Code)
}

func (directory HttpDirectory) notFoundError(code string) error {
	if code == directory.domainNotHostedCode() {
		return reception.DomainNotHosted{}
	}
	return reception.UnknownAddress{}
}

func (directory HttpDirectory) unexpected(response *http.Response) (reception.Address, error) {
	return reception.Address{}, UnexpectedStatus{Service: directory.serviceName(), Status: response.StatusCode}
}

func (HttpDirectory) domainNotHostedCode() string {
	return "domain_not_hosted"
}

func (HttpDirectory) serviceName() string {
	return "directory"
}
