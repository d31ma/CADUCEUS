package inbound

import "strings"

type Addresses struct{}

func (Addresses) Normal(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func (addresses Addresses) LocalPart(address string) string {
	localPart, _, _ := strings.Cut(addresses.Normal(address), "@")
	return localPart
}

func (addresses Addresses) Domain(address string) string {
	_, domain, _ := strings.Cut(addresses.Normal(address), "@")
	return domain
}
