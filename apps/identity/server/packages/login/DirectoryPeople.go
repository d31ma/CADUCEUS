package login

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"mesail.dev/packages/oidc"
)

type DirectoryPeople struct {
	Client  *http.Client
	BaseUrl string
	Signer  Signer
}

func (people DirectoryPeople) Person(personId string) (Person, error) {
	caller, cancel := context.WithTimeout(context.Background(), people.totalTime())
	defer cancel()
	first := people.try(caller, personId)
	return people.answer(people.againWhen(first.Retryable(), first, caller, personId), personId)
}

func (people DirectoryPeople) againWhen(retry bool, first DirectoryReply, caller context.Context, personId string) DirectoryReply {
	if !retry {
		return first
	}
	people.wait(caller)
	return people.try(caller, personId)
}

func (people DirectoryPeople) wait(caller context.Context) {
	select {
	case <-time.After(people.pause()):
	case <-caller.Done():
	}
}

func (people DirectoryPeople) try(caller context.Context, personId string) DirectoryReply {
	request, err := people.request(caller, personId)
	if err != nil {
		return DirectoryReply{Failed: err}
	}
	return people.send(request)
}

func (people DirectoryPeople) send(request *http.Request) DirectoryReply {
	response, err := people.Client.Do(request)
	if err != nil {
		return DirectoryReply{Failed: oidc.SessionUnavailable{}, Unreachable: true}
	}
	defer response.Body.Close()
	return people.read(response)
}

func (people DirectoryPeople) read(response *http.Response) DirectoryReply {
	body, err := io.ReadAll(io.LimitReader(response.Body, people.maximumBytes()))
	if err != nil {
		return DirectoryReply{Failed: oidc.SessionUnavailable{}, Unreachable: true}
	}
	return DirectoryReply{Status: response.StatusCode, Body: body}
}

func (people DirectoryPeople) request(caller context.Context, personId string) (*http.Request, error) {
	issued, err := people.Signer.Sign(ServiceTokenIssuer{}.InstallWideToken(oidc.InstallWide{}.DomainId()))
	if err != nil {
		return nil, oidc.SessionUnavailable{}
	}
	request, err := http.NewRequestWithContext(caller, http.MethodGet, people.BaseUrl+people.path()+url.PathEscape(personId), nil)
	if err != nil {
		return nil, oidc.SessionUnavailable{}
	}
	request.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	return request, nil
}

func (people DirectoryPeople) answer(reply DirectoryReply, personId string) (Person, error) {
	if reply.Failed != nil {
		return Person{}, reply.Failed
	}
	return people.found(reply, personId)
}

func (people DirectoryPeople) found(reply DirectoryReply, personId string) (Person, error) {
	if reply.Status == http.StatusNotFound {
		return Person{PersonId: personId}, nil
	}
	return people.decoded(reply)
}

func (people DirectoryPeople) decoded(reply DirectoryReply) (Person, error) {
	person := Person{}
	if reply.Status != http.StatusOK || json.Unmarshal(reply.Body, &person) != nil {
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

func (DirectoryPeople) pause() time.Duration {
	return 200 * time.Millisecond
}

func (DirectoryPeople) totalTime() time.Duration {
	return 11500 * time.Millisecond
}
