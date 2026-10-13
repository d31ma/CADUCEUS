// CategoryMappingService keeps CADUCEUS' stable system roles separate from
// a mailbox's physical folder and label paths, per exact mailbox domain.
// @Service
type CategoryMappingService struct {
	// @Needs
	usersHelper *UsersHelper
	// @Needs
	emailsHelper *EmailsHelper
}

// ForOwner answers the owner's role mappings (Found false when the owner has
// no user record).
func (service *CategoryMappingService) ForOwner(ownerEmail string) (CategoryMappings, error) {
	_, user, err := service.usersHelper.FindByEmail(ownerEmail)
	return CategoryMappings{}.Of(user), err
}

// ListAccounts answers the owner's physical categories per allowed domain,
// or nil when the owner has no user record.
func (service *CategoryMappingService) ListAccounts(ownerEmail string, allowedDomains []string) ([]map[string]any, error) {
	return CategoryListing{Users: service.usersHelper, Emails: service.emailsHelper}.Accounts(ownerEmail, allowedDomains)
}

// CreateCategory adds one folder or label to the owner's own catalog.
func (service *CategoryMappingService) CreateCategory(ownerEmail string, input any) (map[string]any, *CategoryRefusal, error) {
	return CategoryListing{Users: service.usersHelper, Emails: service.emailsHelper}.Create(ownerEmail, input)
}
