package accounts

import "context"

// CapabilityValidator adds capabilities supplied by a provider-native API
// only after the underlying mailbox transport has validated successfully.
// It never turns a failed OAuth/IMAP validation into a usable account.
type CapabilityValidator struct {
	Base  Validator
	Extra Capabilities
}

func (validator CapabilityValidator) Validate(ctx context.Context, request ValidationRequest) (Capabilities, error) {
	if validator.Base == nil {
		return Capabilities{}, ErrUnsupported
	}
	capabilities, err := validator.Base.Validate(ctx, request)
	if err != nil {
		return Capabilities{}, err
	}
	capabilities.Contacts = capabilities.Contacts || validator.Extra.Contacts
	capabilities.Calendar = capabilities.Calendar || validator.Extra.Calendar
	capabilities.Labels = capabilities.Labels || validator.Extra.Labels
	return capabilities, nil
}
