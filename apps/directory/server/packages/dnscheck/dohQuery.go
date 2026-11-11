package dnscheck

import (
	"fmt"
	"net/http"
	"net/url"
)

type dohQuery struct {
	endpoint string
}

func (query dohQuery) request(name string, kind recordType) (*http.Request, error) {
	address, err := query.address(name, kind)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("dnscheck: build request for %q: %w", name, err)
	}
	request.Header.Set(query.acceptHeader(), query.dnsJson())
	return request, nil
}

func (query dohQuery) address(name string, kind recordType) (string, error) {
	address, err := url.Parse(query.endpoint)
	if err != nil {
		return "", fmt.Errorf("dnscheck: parse endpoint: %w", err)
	}
	values := address.Query()
	values.Set(query.nameParameter(), name)
	values.Set(query.typeParameter(), kind.name)
	address.RawQuery = values.Encode()
	return address.String(), nil
}

func (dohQuery) nameParameter() string {
	return "name"
}

func (dohQuery) typeParameter() string {
	return "type"
}

func (dohQuery) acceptHeader() string {
	return "Accept"
}

func (dohQuery) dnsJson() string {
	return "application/dns-json"
}
