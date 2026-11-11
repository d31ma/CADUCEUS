package mailin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Directory struct {
	Client    *http.Client
	BaseUrl   string
	Authorize func(realm string) (string, error)
}

func (Directory) InstallWideRealm() string {
	return "mesail-operators"
}

func (directory Directory) Domain(domainId string) (Domain, bool, error) {
	request, err := directory.request(domainId)
	if err != nil {
		return Domain{}, false, err
	}
	response, err := directory.Client.Do(request)
	if err != nil {
		return Domain{}, false, err
	}
	defer response.Body.Close()
	return directory.answer(response)
}

func (directory Directory) request(domainId string) (*http.Request, error) {
	authorization, err := directory.Authorize(directory.InstallWideRealm())
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodGet, directory.address(domainId), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", authorization)
	return request, nil
}

func (directory Directory) address(domainId string) string {
	return strings.TrimRight(directory.BaseUrl, "/") + "/api/v1/directory/domains/" + url.PathEscape(strings.ToLower(domainId))
}

func (directory Directory) answer(response *http.Response) (Domain, bool, error) {
	switch response.StatusCode {
	case http.StatusOK:
		return directory.decoded(response.Body)
	case http.StatusNotFound:
		return Domain{}, false, nil
	}
	return Domain{}, false, fmt.Errorf("the directory answered %d", response.StatusCode)
}

func (directory Directory) decoded(body io.Reader) (Domain, bool, error) {
	domain := Domain{}
	err := json.NewDecoder(io.LimitReader(body, directory.readLimit())).Decode(&domain)
	return domain, err == nil, err
}

func (Directory) readLimit() int64 {
	return 1048576
}
