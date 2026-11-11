package dnscheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type DohLookup struct {
	endpoint string
	client   *http.Client
}

func NewDohLookup(endpoint string, client *http.Client) DohLookup {
	return DohLookup{endpoint: endpoint, client: client}
}

func (lookup DohLookup) Mx(name string) ([]string, error) {
	values, err := lookup.values(name, recordTypes{}.mx())
	if err != nil {
		return nil, err
	}
	return lookup.mxHosts(values), nil
}

func (lookup DohLookup) Txt(name string) ([]string, error) {
	values, err := lookup.values(name, recordTypes{}.txt())
	if err != nil {
		return nil, err
	}
	return lookup.texts(values), nil
}

func (lookup DohLookup) values(name string, kind recordType) ([]string, error) {
	response, err := lookup.fetch(name, kind)
	if err != nil {
		return nil, err
	}
	err = response.check(name)
	if err != nil {
		return nil, err
	}
	return response.values(kind), nil
}

func (lookup DohLookup) fetch(name string, kind recordType) (dohResponse, error) {
	request, err := dohQuery{endpoint: lookup.endpoint}.request(name, kind)
	if err != nil {
		return dohResponse{}, err
	}
	response, err := lookup.client.Do(request)
	if err != nil {
		return dohResponse{}, fmt.Errorf("dnscheck: query %q: %w", name, err)
	}
	parsed, err := lookup.read(name, response)
	return parsed, errors.Join(err, response.Body.Close())
}

func (lookup DohLookup) read(name string, response *http.Response) (dohResponse, error) {
	err := lookup.checkHttpStatus(name, response.StatusCode)
	if err != nil {
		return dohResponse{}, err
	}
	return lookup.decode(name, response.Body)
}

func (lookup DohLookup) checkHttpStatus(name string, status int) error {
	if status == http.StatusOK {
		return nil
	}
	return LookupFailed{Name: name, Status: status}
}

func (lookup DohLookup) decode(name string, body io.Reader) (dohResponse, error) {
	var parsed dohResponse
	err := json.NewDecoder(io.LimitReader(body, lookup.maxResponseBytes())).Decode(&parsed)
	if err != nil {
		return dohResponse{}, fmt.Errorf("dnscheck: decode answer for %q: %w", name, err)
	}
	return parsed, nil
}

func (lookup DohLookup) mxHosts(values []string) []string {
	hosts := []string{}
	for _, value := range values {
		hosts = append(hosts, answerData{}.mxHost(value))
	}
	return hosts
}

func (lookup DohLookup) texts(values []string) []string {
	texts := []string{}
	for _, value := range values {
		texts = append(texts, answerData{}.text(value))
	}
	return texts
}

func (DohLookup) maxResponseBytes() int64 {
	return 1 << 20
}
