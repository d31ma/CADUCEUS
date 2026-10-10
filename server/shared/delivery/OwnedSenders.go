// OwnedSenders resolves the exact addresses an authenticated mailbox may
// use as its From identity: its primary address and its aliases. Domain
// authority is deliberately not enough, so another user of the same hosted
// domain can never impersonate the mailbox.
type OwnedSenders struct {
	Users *UsersHelper
}

// Owned answers the owner's sender addresses: the owner alone when it has no
// user record.
func (senders OwnedSenders) Owned(ownerEmail string) ([]string, error) {
	security := Security{}
	owner := security.NormalizeEmailAddress(ownerEmail)
	if owner == "" {
		return []string{}, nil
	}
	_, user, err := senders.Users.FindByEmail(owner)
	if err != nil || user == nil {
		return []string{owner}, err
	}
	addresses := []string{}
	candidates := append([]string{RecordValues{}.Text(user["email"])}, DeliveryFields{}.Strings(user["aliases"])...)
	for _, candidate := range candidates {
		normalized := security.NormalizeEmailAddress(candidate)
		if normalized != "" && !(TextValue{}).Includes(addresses, normalized) {
			addresses = append(addresses, normalized)
		}
	}
	return addresses, nil
}

// Resolved answers the requested From address (or fallback when none is
// requested) when the mailbox owns it, else "".
func (OwnedSenders) Resolved(requested any, allowed []string, fallback string) string {
	value, isText := requested.(string)
	if !isText {
		value = fallback
	}
	normalized := Security{}.NormalizeEmailAddress(value)
	if normalized != "" && (TextValue{}).Includes(allowed, normalized) {
		return normalized
	}
	return ""
}

// Sender answers the mailbox's resolved From address, or "".
func (senders OwnedSenders) Sender(ownerEmail string, requested any) (string, error) {
	allowed, err := senders.Owned(ownerEmail)
	if err != nil {
		return "", err
	}
	return senders.Resolved(requested, allowed, ownerEmail), nil
}
