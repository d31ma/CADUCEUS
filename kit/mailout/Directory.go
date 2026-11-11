package mailout

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Directory struct {
	Client  ServiceClient
	BaseUrl string
}

func (directory Directory) Domain(domainId string, realm string) (Domain, bool, error) {
	address := strings.TrimRight(directory.BaseUrl, "/") + "/api/v1/directory/domains/" + url.PathEscape(strings.ToLower(domainId))
	domain := Domain{}
	status, err := directory.Client.Get(address, realm, &domain)
	if err != nil {
		return Domain{}, false, err
	}
	return directory.answer(status, domain)
}

func (Directory) answer(status int, domain Domain) (Domain, bool, error) {
	switch status {
	case http.StatusOK:
		return domain, true, nil
	case http.StatusNotFound:
		return Domain{}, false, nil
	default:
		return Domain{}, false, fmt.Errorf("directory answered %d", status)
	}
}
