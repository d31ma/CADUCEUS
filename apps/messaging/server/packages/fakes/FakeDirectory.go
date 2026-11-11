package fakes

import (
	"strings"

	"mesail.dev/packages/core"
)

type FakeDirectory struct {
	Resolved      map[string]core.Resolved
	Entries       map[string]core.AddressEntry
	Authorization []string
	Failure       error
}

func NewFakeDirectory() *FakeDirectory {
	return &FakeDirectory{Resolved: map[string]core.Resolved{}, Entries: map[string]core.AddressEntry{}}
}

func (directory *FakeDirectory) Member(address string, memberId string, displayName string) {
	directory.Resolved[address] = core.Resolved{Address: address, DisplayName: displayName, Kind: "member", MemberId: memberId}
	directory.Entries[address] = core.AddressEntry{Address: address, DomainId: directory.domainOf(address), Kind: "member", TargetId: memberId}
}

func (directory *FakeDirectory) Channel(address string, channelId string) {
	directory.Resolved[address] = core.Resolved{Address: address, Kind: "channel", ChannelId: channelId}
	directory.Entries[address] = core.AddressEntry{Address: address, DomainId: directory.domainOf(address), Kind: "channel", TargetId: channelId}
}

func (directory *FakeDirectory) OtherDomain(address string) {
	directory.Resolved[address] = core.Resolved{Address: address, Kind: "otherDomain"}
}

func (directory *FakeDirectory) Resolve(authorization string, addresses []string) ([]core.Resolved, error) {
	directory.Authorization = append(directory.Authorization, authorization)
	resolved := []core.Resolved{}
	for _, address := range addresses {
		resolved = append(resolved, directory.resolved(address))
	}
	return resolved, directory.Failure
}

func (directory *FakeDirectory) Address(domainId string, address string) (core.AddressEntry, bool, error) {
	entry, found := directory.Entries[address]
	return entry, found && entry.DomainId == domainId, directory.Failure
}

func (directory *FakeDirectory) resolved(address string) core.Resolved {
	resolved, found := directory.Resolved[address]
	if found {
		return resolved
	}
	return core.Resolved{Address: address, Kind: "external"}
}

func (directory *FakeDirectory) domainOf(address string) string {
	_, domain, _ := strings.Cut(address, "@")
	return domain
}
