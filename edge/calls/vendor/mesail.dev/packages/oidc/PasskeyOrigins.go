package oidc

import (
	"errors"
	"net/url"
	"slices"
	"strings"
)

type PasskeyOrigins struct {
	Text string
}

func (origins PasskeyOrigins) List() []string {
	return strings.Fields(origins.Text)
}

func (origins PasskeyOrigins) Field() string {
	return strings.Join(origins.List(), " ")
}

func (origins PasskeyOrigins) RelyingPartyId() string {
	secure := origins.hosts(origins.secureScheme())
	local := origins.hosts(origins.plainScheme())
	return origins.first(append(secure, local...))
}

func (origins PasskeyOrigins) Problem() error {
	relyingPartyId := origins.RelyingPartyId()
	missing := origins.missing(relyingPartyId)
	otherHost := origins.allMatch(origins.hosts(origins.secureScheme()), relyingPartyId)
	notLocal := origins.allMatch(origins.hosts(origins.plainScheme()), origins.localHost())
	return errors.Join(missing, otherHost, notLocal)
}

func (origins PasskeyOrigins) hosts(scheme string) []string {
	hosts := []string{}
	for _, origin := range origins.List() {
		hosts = append(hosts, origins.hostFor(origin, scheme)...)
	}
	return hosts
}

func (PasskeyOrigins) hostFor(origin string, scheme string) []string {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != scheme {
		return nil
	}
	return []string{parsed.Hostname()}
}

func (PasskeyOrigins) first(hosts []string) string {
	if len(hosts) == 0 {
		return ""
	}
	return hosts[0]
}

func (PasskeyOrigins) missing(relyingPartyId string) error {
	if relyingPartyId == "" {
		return NotConfigured{}
	}
	return nil
}

func (PasskeyOrigins) allMatch(hosts []string, wanted string) error {
	if slices.ContainsFunc(hosts, func(host string) bool { return host != wanted }) {
		return NotConfigured{}
	}
	return nil
}

func (PasskeyOrigins) secureScheme() string {
	return "https"
}

func (PasskeyOrigins) plainScheme() string {
	return "http"
}

func (PasskeyOrigins) localHost() string {
	return "localhost"
}
