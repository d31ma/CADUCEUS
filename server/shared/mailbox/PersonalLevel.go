// PersonalLevel reads who a message is visibly addressed to and how
// personal a conversation is for one mailbox, with Mailspring's levels.
type PersonalLevel struct{}

// ParseAddressList answers the unique lower-case addresses in an RFC
// address-list value or a list of them, without display names.
func (PersonalLevel) ParseAddressList(value any) []string {
	values, isList := value.([]any)
	if !isList {
		values = []any{value}
	}
	addresses := MailAddresses{}
	found := []string{}
	for _, item := range values {
		found = append(found, addresses.Found(item)...)
	}
	return addresses.Unique(found)
}

// RecipientsFromHeaders answers the visible To, Cc and Bcc addresses of a
// message; the SMTP envelope recipient is used only when the message has no
// recipient headers at all.
func (level PersonalLevel) RecipientsFromHeaders(headers map[string]any, envelopeRecipient string) map[string][]string {
	to := level.ParseAddressList(headers["to"])
	cc := level.ParseAddressList(headers["cc"])
	bcc := level.ParseAddressList(headers["bcc"])
	if len(to)+len(cc)+len(bcc) == 0 {
		return map[string][]string{"to": level.ParseAddressList(envelopeRecipient), "cc": {}, "bcc": {}}
	}
	return map[string][]string{"to": to, "cc": cc, "bcc": bcc}
}

// MessageParticipants answers the sender and visible recipients of one
// stored message (the envelope recipient for historical records).
func (level PersonalLevel) MessageParticipants(email map[string]any) []string {
	recipients := []string{}
	for _, field := range []string{"to", "cc", "bcc"} {
		recipients = append(recipients, level.ParseAddressList(email[field])...)
	}
	if len(recipients) == 0 {
		recipients = level.ParseAddressList(email["recipient"])
	}
	return MailAddresses{}.Unique(append(level.ParseAddressList(email["sender"]), recipients...))
}

// PersonalLevelForThread answers 2 when the conversation is between the
// mailbox and one other participant, 1 when the mailbox is one of three or
// more, and 0 otherwise.
func (level PersonalLevel) PersonalLevelForThread(emails []map[string]any, mailbox string) int {
	mine := level.ParseAddressList(mailbox)
	if len(mine) == 0 {
		return 0
	}
	participants := map[string]bool{}
	for _, email := range emails {
		for _, participant := range level.MessageParticipants(email) {
			participants[participant] = true
		}
	}
	switch {
	case !participants[mine[0]]:
		return 0
	case len(participants) == 2:
		return 2
	case len(participants) > 2:
		return 1
	}
	return 0
}
