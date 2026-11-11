import (
	"cmp"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mesail.dev/packages/oidc"
)

// @Service
type PresenceService struct {
	// @Needs
	settings *NotifySettingsService
}

func (service *PresenceService) States(domainId string, memberIds []string) (map[string]Presence, error) {
	address := service.settings.Options().Presence.Url
	return service.statesFrom(address, domainId, memberIds)
}

func (service *PresenceService) statesFrom(address string, domainId string, memberIds []string) (map[string]Presence, error) {
	if address == "" || len(memberIds) == 0 {
		return map[string]Presence{}, nil
	}
	return service.fetch(address, domainId, memberIds)
}

func (service *PresenceService) fetch(address string, domainId string, memberIds []string) (map[string]Presence, error) {
	authorization, tokenFailure := oidc.ServiceTokensFactory{}.Configured().Authorization(domainId)
	request, requestFailure := http.NewRequest(http.MethodGet, service.query(address, memberIds), nil)
	response, failure := service.send(request, authorization, cmp.Or(tokenFailure, requestFailure))
	failure = service.checked(response, failure)
	presences, failure := service.decode(response, failure)
	service.closeBody(response)
	return service.byMember(presences), failure
}

func (service *PresenceService) query(address string, memberIds []string) string {
	values := url.Values{}
	values.Set(service.memberIdsKey(), strings.Join(memberIds, ","))
	return strings.TrimRight(address, "/") + service.route() + "?" + values.Encode()
}

func (service *PresenceService) send(request *http.Request, authorization string, failure error) (*http.Response, error) {
	if failure != nil {
		return nil, failure
	}
	request.Header.Set("Authorization", authorization)
	return (&http.Client{Timeout: service.timeout()}).Do(request)
}

func (service *PresenceService) checked(response *http.Response, failure error) error {
	if failure == nil && response.StatusCode != http.StatusOK {
		return &PresenceUnavailable{Status: response.StatusCode}
	}
	return failure
}

func (service *PresenceService) decode(response *http.Response, failure error) ([]Presence, error) {
	presences := []Presence{}
	if failure != nil {
		return presences, failure
	}
	decodeFailure := json.NewDecoder(response.Body).Decode(&presences)
	return presences, decodeFailure
}

func (service *PresenceService) closeBody(response *http.Response) {
	if response != nil {
		response.Body.Close()
	}
}

func (service *PresenceService) byMember(presences []Presence) map[string]Presence {
	keyed := map[string]Presence{}
	for _, presence := range presences {
		keyed[presence.MemberId] = presence
	}
	return keyed
}

func (service *PresenceService) route() string {
	return "/api/v1/realtime/presence"
}

func (service *PresenceService) memberIdsKey() string {
	return "memberIds"
}

func (service *PresenceService) timeout() time.Duration {
	return 10 * time.Second
}
