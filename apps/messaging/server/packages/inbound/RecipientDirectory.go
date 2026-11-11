package inbound

import "mesail.dev/packages/core"

type RecipientDirectory struct {
	directory core.Directory
	members   core.MemberStore
}

func NewRecipientDirectory(stores core.Stores, ports core.Ports) RecipientDirectory {
	return RecipientDirectory{directory: ports.Directory, members: stores.Members}
}

func (recipients RecipientDirectory) Entries(email ReceivedEmail) ([]core.AddressEntry, error) {
	return recipients.entries(email.DomainId, email.Mail.Rcpts)
}

func (recipients RecipientDirectory) Members(email ReceivedEmail, conversationId string) ([]core.Participant, error) {
	entries, err := recipients.Entries(email)
	if err != nil {
		return nil, err
	}
	return recipients.MemberParticipants(email.DomainId, conversationId, entries)
}

func (recipients RecipientDirectory) MemberParticipants(domainId string, conversationId string, entries []core.AddressEntry) ([]core.Participant, error) {
	return recipients.participants(domainId, conversationId, recipients.memberEntries(entries))
}

func (recipients RecipientDirectory) entries(domainId string, rcpts []string) ([]core.AddressEntry, error) {
	entries := []core.AddressEntry{}
	for _, rcpt := range rcpts {
		entry, found, err := recipients.directory.Address(domainId, Addresses{}.Normal(rcpt))
		if err != nil {
			return nil, Unavailability{}.Of(err)
		}
		entries = append(entries, recipients.ifHosted(domainId, entry, found)...)
	}
	return entries, nil
}

func (RecipientDirectory) ifHosted(domainId string, entry core.AddressEntry, found bool) []core.AddressEntry {
	if !found || entry.DomainId != domainId {
		return nil
	}
	return []core.AddressEntry{entry}
}

func (recipients RecipientDirectory) memberEntries(entries []core.AddressEntry) []core.AddressEntry {
	kept := []core.AddressEntry{}
	for _, entry := range entries {
		kept = append(kept, recipients.ifMember(entry)...)
	}
	return kept
}

func (RecipientDirectory) ifMember(entry core.AddressEntry) []core.AddressEntry {
	kinds := AddressKinds{}
	if entry.Kind != kinds.Member() && entry.Kind != kinds.Alias() {
		return nil
	}
	return []core.AddressEntry{entry}
}

func (recipients RecipientDirectory) participants(domainId string, conversationId string, entries []core.AddressEntry) ([]core.Participant, error) {
	participants := []core.Participant{}
	for _, entry := range entries {
		member, _, err := recipients.members.ByMemberId(domainId, entry.TargetId)
		if err != nil {
			return nil, Unavailability{}.Of(err)
		}
		participants = append(participants, EmailParticipants{}.Member(domainId, conversationId, entry, member))
	}
	return participants, nil
}
