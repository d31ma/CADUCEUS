package dnscheck

import "strings"

type hostName struct{}

func (name hostName) normalize(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), name.rootDot())
}

func (hostName) rootDot() string {
	return "."
}
