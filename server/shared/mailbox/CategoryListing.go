// CategoryListing reads and extends an owner's category catalog through the
// user and email helpers.
type CategoryListing struct {
	Users  *UsersHelper
	Emails *EmailsHelper
}

// Accounts answers the owner's categories per allowed domain, or nil when the
// owner has no user record.
func (listing CategoryListing) Accounts(ownerEmail string, allowedDomains []string) ([]map[string]any, error) {
	_, user, err := listing.Users.FindByEmail(ownerEmail)
	if err != nil || user == nil {
		return nil, err
	}
	emails, err := listing.Emails.List(allowedDomains, EmailListFilters{Folder: "all", OwnerEmail: ownerEmail})
	if err != nil {
		return nil, err
	}
	return CategoryAccounts{}.Listed(user, allowedDomains, emails), nil
}

// Create adds one category to the owner's catalog and answers it.
func (listing CategoryListing) Create(ownerEmail string, input any) (map[string]any, *CategoryRefusal, error) {
	documentId, user, err := listing.Users.FindByEmail(ownerEmail)
	if err != nil {
		return nil, nil, err
	}
	if documentId == "" || user == nil {
		return nil, &CategoryRefusal{NotFound: true, Message: "User not found"}, nil
	}
	catalog, category, refusal := CategoryAccounts{}.Added(user, input)
	if refusal != nil {
		return nil, refusal, nil
	}
	return category, nil, listing.Users.UpdateCategoryCatalog(documentId, catalog)
}
