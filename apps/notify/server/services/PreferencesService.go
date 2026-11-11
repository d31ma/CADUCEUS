import (
	"cmp"
	"errors"

	"mesail.dev/packages/notify"
	"mesail.dev/packages/oidc"
)

// @Service
type PreferencesService struct {
	// @Needs
	callers *CallerService
	// @Needs
	store *MemberSettingsStoreService
}

func (service *PreferencesService) Get(authorization string) (NotifySettings, error) {
	caller, failure := service.callers.Caller(authorization)
	record, failure := service.getUnless(failure, caller)
	return NotifySettings{EmailSummaryAfterMinutes: record.EmailSummaryAfterMinutes, MutedConversationIds: service.orEmpty(record.MutedConversationIds)}, failure
}

func (service *PreferencesService) Put(authorization string, settings NotifySettings) (NotifySettings, error) {
	caller, failure := service.callers.Caller(authorization)
	settings.MutedConversationIds = service.orEmpty(settings.MutedConversationIds)
	refusal := notify.SettingsCheck{}.Check(settings.EmailSummaryAfterMinutes, settings.MutedConversationIds)
	failure = cmp.Or(failure, service.invalid(refusal))
	return settings, service.putUnless(failure, caller, settings)
}

func (service *PreferencesService) getUnless(failure error, caller oidc.Caller) (MemberSettingsRecord, error) {
	if failure != nil {
		return MemberSettingsRecord{}, failure
	}
	return service.store.Get(caller.DomainId, caller.MemberId)
}

func (service *PreferencesService) putUnless(failure error, caller oidc.Caller, settings NotifySettings) error {
	if failure != nil {
		return failure
	}
	return service.store.PutPreferences(caller.DomainId, caller.MemberId, settings)
}

func (service *PreferencesService) invalid(refusal error) error {
	invalid := notify.InvalidSettings{}
	if errors.As(refusal, &invalid) {
		return &InvalidRequest{Field: service.wireFields()[invalid.Field], Reason: invalid.Reason}
	}
	return refusal
}

func (service *PreferencesService) wireFields() map[string]string {
	return map[string]string{"afterMinutes": "emailSummaryAfterMinutes", service.mutedField(): service.mutedField()}
}

func (service *PreferencesService) mutedField() string {
	return "mutedConversationIds"
}

func (service *PreferencesService) orEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
