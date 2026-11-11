package login

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"mesail.dev/packages/oidc"
)

type DirectoryPeople struct {
	Client  *http.Client
	BaseUrl string
	Signer  Signer
}

func (people DirectoryPeople) Person(personId string) (Person, error) {
	request, err := people.request(personId)
	if err != nil {
		return Person{}, err
	}
	response, err := people.Client.Do(request)
	if err != nil {
		return Person{}, oidc.SessionUnavailable{}
	}
	defer response.Body.Close()
	return people.answer(response, personId)
}

func (people DirectoryPeople) request(personId string) (*http.Request, error) {
	issued, err := people.Signer.Sign(ServiceTokenIssuer{}.InstallWideToken(oidc.InstallWide{}.DomainId()))
	if err != nil {
		return nil, oidc.SessionUnavailable{}
	}
	request, err := http.NewRequest(http.MethodGet, people.BaseUrl+people.path()+url.PathEscape(personId), nil)
	if err != nil {
		return nil, oidc.SessionUnavailable{}
	}
	request.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	return request, nil
}

func (people DirectoryPeople) answer(response *http.Response, personId string) (Person, error) {
	if response.StatusCode == http.StatusNotFound {
		return Person{PersonId: personId}, nil
	}
	return people.decoded(response)
}

func (people DirectoryPeople) decoded(response *http.Response) (Person, error) {
	person := Person{}
	raw, err := io.ReadAll(io.LimitReader(response.Body, people.maximumBytes()))
	if err != nil || response.StatusCode != http.StatusOK || json.Unmarshal(raw, &person) != nil {
		return Person{}, oidc.SessionUnavailable{}
	}
	return person, nil
}

func (DirectoryPeople) path() string {
	return "/api/v1/directory/people/"
}

func (DirectoryPeople) maximumBytes() int64 {
	return 1 << 20
}
