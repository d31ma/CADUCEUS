// SenderAddresses resolves the exact addresses a mailbox may send as: its
// primary address and its aliases. Domain authority is not enough; another
// user of the same hosted domain must never be impersonated.
type SenderAddresses struct {
	Users *UsersHelper
}

// Owned answers the owner's sender addresses (just the owner without a user
// record; none for an invalid owner).
func (addresses SenderAddresses) Owned(ownerEmail string) ([]string, error) {
	owner := Security{}.NormalizeEmailAddress(ownerEmail)
	if owner == "" {
		return []string{}, nil
	}
	_, user, err := addresses.Users.FindByEmail(owner)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return []string{owner}, nil
	}
	aliases, _ := RecordValues{}.List(user["aliases"])
	owned := []string{}
	for _, address := range append([]any{user["email"]}, aliases...) {
		normalized := Security{}.NormalizeEmailAddress(TextValue{}.Of(address))
		if normalized != "" && !(PreferenceChoices{}).Includes(owned, normalized) {
			owned = append(owned, normalized)
		}
	}
	return owned, nil
}

// Resolved answers the requested From address (or fallback when none is
// requested) when the mailbox owns it, else "".
func (SenderAddresses) Resolved(requested any, allowed []string, fallback string) string {
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
func (addresses SenderAddresses) Sender(ownerEmail string, requested any) (string, error) {
	allowed, err := addresses.Owned(ownerEmail)
	if err != nil {
		return "", err
	}
	return addresses.Resolved(requested, allowed, ownerEmail), nil
}
