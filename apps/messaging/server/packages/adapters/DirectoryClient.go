package adapters

import (
	"net/http"
	"net/url"
	"strings"

	"mesail.dev/packages/core"
)

type DirectoryClient struct {
	baseUrl string
	tokens  ServiceAuthorizer
	call    JsonCall
}

func NewDirectoryClient(baseUrl string, tokens ServiceAuthorizer, call JsonCall) DirectoryClient {
	return DirectoryClient{baseUrl: strings.TrimSuffix(baseUrl, "/"), tokens: tokens, call: call}
}

func (client DirectoryClient) Resolve(authorization string, addresses []string) ([]core.Resolved, error) {
	response := ResolveResponse{Recipients: []core.Resolved{}}
	request := JsonRequest{Method: http.MethodPost, Url: client.baseUrl + client.resolvePath(), Authorization: authorization, Body: ResolveRequest{Addresses: addresses}}
	found, err := client.call.Send(request, &response)
	if err != nil {
		return nil, err
	}
	return response.Recipients, client.refusedUnless(found)
}

func (client DirectoryClient) Address(domainId string, address string) (core.AddressEntry, bool, error) {
	authorization, err := client.tokens.Authorization(domainId)
	if err != nil {
		return core.AddressEntry{}, false, &core.Unavailable{Message: "no service token for the domain", Cause: err}
	}
	entry := core.AddressEntry{}
	request := JsonRequest{Method: http.MethodGet, Url: client.baseUrl + client.addressPath() + url.PathEscape(address), Authorization: authorization}
	found, err := client.call.Send(request, &entry)
	return entry, client.sameDomain(found, entry, domainId), err
}

func (DirectoryClient) refusedUnless(found bool) error {
	if found {
		return nil
	}
	return &core.Unavailable{Message: "the directory does not know the resolve route"}
}

func (DirectoryClient) sameDomain(found bool, entry core.AddressEntry, domainId string) bool {
	return found && strings.EqualFold(entry.DomainId, domainId)
}

func (DirectoryClient) resolvePath() string {
	return "/api/v1/directory/recipients/resolve"
}

func (DirectoryClient) addressPath() string {
	return "/api/v1/directory/addresses/"
}
