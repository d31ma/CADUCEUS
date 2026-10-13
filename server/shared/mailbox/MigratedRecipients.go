import "strings"

// MigratedRecipients presents mail received before a domain move under the
// new domain, keeping the address it was delivered to as originalRecipient.
type MigratedRecipients struct {
	Migrations *DomainMigrationsHelper
}

// Presented answers the emails with their recipients moved to the new
// domain of the first migration that matches each one.
func (recipients MigratedRecipients) Presented(emails []map[string]any) ([]map[string]any, error) {
	migrations, err := recipients.Migrations.List()
	if err != nil || len(migrations) == 0 {
		return emails, err
	}
	presented := []map[string]any{}
	for _, email := range emails {
		presented = append(presented, recipients.rewritten(email, migrations))
	}
	return presented, nil
}

// PresentedOne answers one email presented with Presented.
func (recipients MigratedRecipients) PresentedOne(email map[string]any) (map[string]any, error) {
	migrations, err := recipients.Migrations.List()
	if err != nil {
		return nil, err
	}
	return recipients.rewritten(email, migrations), nil
}

func (MigratedRecipients) rewritten(email map[string]any, migrations []map[string]any) map[string]any {
	text := TextValue{}
	recipient := text.Of(email["recipient"])
	for _, migration := range migrations {
		fromSuffix := "@" + text.Of(migration["fromDomain"])
		if strings.HasSuffix(recipient, fromSuffix) {
			rewritten := text.Copy(email)
			rewritten["recipient"] = strings.TrimSuffix(recipient, fromSuffix) + "@" + text.Of(migration["toDomain"])
			rewritten["originalRecipient"] = text.Or(email["originalRecipient"], email["recipient"])
			return rewritten
		}
	}
	return email
}
