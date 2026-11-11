//go:build responsive

package responsive

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
)

type IdentityFake struct {
	HostedDomains []string
}

type fakeMe struct {
	MemberId       string   `json:"memberId"`
	DomainId       string   `json:"domainId"`
	DisplayName    string   `json:"displayName"`
	PrimaryAddress string   `json:"primaryAddress"`
	Roles          []string `json:"roles"`
}

type fakeSession struct {
	Me          fakeMe `json:"me"`
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
	SessionKey  string `json:"sessionKey"`
}

type fakeApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type fakeSignInRequest struct {
	Address     string `json:"address"`
	Password    string `json:"password"`
	OneTimeCode string `json:"oneTimeCode"`
}

type fakeRefreshRequest struct {
	SessionKey string `json:"sessionKey"`
}

const (
	sessionPath         = "/api/v1/identity/session"
	sessionRefreshPath  = "/api/v1/identity/session/refresh"
	rightPassword       = "right"
	rateLimitedPassword = "busy"
	sessionKeyPrefix    = "key-"
)

func NewIdentityFake() *IdentityFake {
	return &IdentityFake{HostedDomains: []string{"example.com", "example.org"}}
}

func (fake *IdentityFake) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.Method == http.MethodPost && request.URL.Path == sessionPath:
		fake.signIn(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == sessionRefreshPath:
		fake.refresh(writer, request)
	case request.Method == http.MethodDelete && request.URL.Path == sessionPath:
		writer.WriteHeader(http.StatusNoContent)
	default:
		writeJson(writer, http.StatusNotFound, fakeApiError{Code: "not_found", Message: "not found"})
	}
}

func (fake *IdentityFake) signIn(writer http.ResponseWriter, request *http.Request) {
	var signIn fakeSignInRequest
	_ = json.NewDecoder(request.Body).Decode(&signIn)
	domain := domainOfAddress(signIn.Address)
	switch {
	case signIn.Password == rateLimitedPassword:
		writeJson(writer, http.StatusTooManyRequests, fakeApiError{Code: "rate_limited", Message: "Too many tries."})
	case signIn.Password == rightPassword && fake.hosts(domain):
		writeJson(writer, http.StatusOK, fake.sessionFor(domain))
	default:
		writeJson(writer, http.StatusUnauthorized, fakeApiError{Code: "unauthenticated", Message: "Sign-in failed."})
	}
}

func (fake *IdentityFake) refresh(writer http.ResponseWriter, request *http.Request) {
	var refresh fakeRefreshRequest
	_ = json.NewDecoder(request.Body).Decode(&refresh)
	domain := strings.TrimPrefix(refresh.SessionKey, sessionKeyPrefix)
	if !strings.HasPrefix(refresh.SessionKey, sessionKeyPrefix) || !fake.hosts(domain) {
		writeJson(writer, http.StatusUnauthorized, fakeApiError{Code: "unauthenticated", Message: "Sign-in failed."})
		return
	}
	writeJson(writer, http.StatusOK, fake.sessionFor(domain))
}

func (fake *IdentityFake) hosts(domain string) bool {
	return slices.Contains(fake.HostedDomains, domain)
}

func (fake *IdentityFake) sessionFor(domain string) fakeSession {
	return fakeSession{
		Me: fakeMe{
			MemberId:       "member-" + domain,
			DomainId:       domain,
			DisplayName:    "Ana Lima",
			PrimaryAddress: "ana@" + domain,
			Roles:          []string{"mesail-member"},
		},
		AccessToken: "token-" + domain,
		ExpiresIn:   60,
		SessionKey:  sessionKeyPrefix + domain,
	}
}

func domainOfAddress(address string) string {
	_, domain, found := strings.Cut(address, "@")
	if !found {
		return ""
	}
	return strings.ToLower(domain)
}

func writeJson(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
