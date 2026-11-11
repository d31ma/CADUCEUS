package inbound

import "mesail.dev/packages/core"

type ConversationStarter struct {
	stores core.Stores
	ports  core.Ports
}

func NewConversationStarter(stores core.Stores, ports core.Ports) ConversationStarter {
	return ConversationStarter{stores: stores, ports: ports}
}

func (starter ConversationStarter) Start(email ReceivedEmail) (Placement, error) {
	entries, err := NewRecipientDirectory(starter.stores, starter.ports).Entries(email)
	if err != nil {
		return Placement{}, err
	}
	channel, found := starter.firstChannel(entries)
	return starter.startIn(email, entries, channel, found)
}

func (ConversationStarter) firstChannel(entries []core.AddressEntry) (core.AddressEntry, bool) {
	for _, entry := range entries {
		if entry.Kind == (AddressKinds{}).Channel() {
			return entry, true
		}
	}
	return core.AddressEntry{}, false
}

func (starter ConversationStarter) startIn(email ReceivedEmail, entries []core.AddressEntry, channel core.AddressEntry, found bool) (Placement, error) {
	if found {
		return NewChannelMail(starter.stores).Place(email, channel.TargetId)
	}
	return NewDirectMail(starter.stores, starter.ports).Place(email, entries)
}
