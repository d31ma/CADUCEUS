// MailActionService applies, undoes and redoes durable, reversible bulk
// mail actions for the exact mailbox owner.
// @Service
type MailActionService struct {
	// @Needs
	mailActionsHelper *MailActionsHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	categoryMappingService *CategoryMappingService
}

// Apply journals and applies one validated action.
func (service *MailActionService) Apply(caller RouteCaller, input MailActionInput, now int64) (map[string]any, error) {
	return service.actions().Apply(caller, input, now)
}

// Undo restores what an applied action replaced.
func (service *MailActionService) Undo(caller RouteCaller, id string, now int64) (map[string]any, error) {
	return service.actions().Undo(caller, id, now)
}

// Redo applies an undone action again.
func (service *MailActionService) Redo(caller RouteCaller, id string, now int64) (map[string]any, error) {
	return service.actions().Redo(caller, id, now)
}

// Mappings answers the owner's category role mappings.
func (service *MailActionService) Mappings(ownerEmail string) (CategoryMappings, error) {
	return service.categoryMappingService.ForOwner(ownerEmail)
}

func (service *MailActionService) actions() MailActions {
	return MailActions{
		Actions: service.mailActionsHelper, Emails: service.emailsHelper, Categories: service.categoryMappingService,
		Provider: UnifiedMail{Accounts: ProviderAccounts{}.FromSettings()},
	}
}
