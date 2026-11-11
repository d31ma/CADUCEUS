package events

type Types struct{}

func (Types) MemberUpserted() string       { return "member.upserted" }
func (Types) ChannelCreated() string       { return "channel.created" }
func (Types) ChannelMemberAdded() string   { return "channel.member.added" }
func (Types) ChannelMemberRemoved() string { return "channel.member.removed" }
func (Types) ChannelArchived() string      { return "channel.archived" }
func (Types) MessageCreated() string       { return "message.created" }
func (Types) MessageEdited() string        { return "message.edited" }
func (Types) MessageDeleted() string       { return "message.deleted" }
func (Types) ReactionChanged() string      { return "reaction.changed" }
func (Types) ReadPositionMoved() string    { return "read-position.moved" }
func (Types) EmailSendRequested() string   { return "email.send.requested" }
func (Types) EmailDeliveryChanged() string { return "email.delivery.changed" }
func (Types) EmailReceived() string        { return "email.received" }
func (Types) PresenceChanged() string      { return "presence.changed" }
