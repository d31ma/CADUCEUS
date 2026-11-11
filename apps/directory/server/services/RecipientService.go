import (
	"cmp"
	"slices"

	"mesail.dev/packages/recipients"
)

// @Service
type RecipientService struct {
	// @Needs
	tenants *TenantService
	// @Needs
	lookup *RecipientLookupService
}

func (service *RecipientService) Resolve(admission Admission, request ResolveRequest) (ResolveResponse, error) {
	addresses, invalid := service.normalized(request.Addresses)
	refusal := cmp.Or(admission.Refusal, invalid)
	hosted, hostedFailure := service.tenants.Hosted()
	resolved, failure := service.resolveAll(admission.Caller.DomainId, service.validOnly(refusal, addresses), hosted)
	return ResolveResponse{Recipients: resolved}, cmp.Or(refusal, hostedFailure, failure)
}

func (service *RecipientService) normalized(raw []string) ([]string, error) {
	addresses, invalid := service.each(raw)
	refusal := service.refusal(len(raw), invalid)
	slices.Sort(addresses)
	return slices.Compact(addresses), refusal
}

func (service *RecipientService) each(raw []string) ([]string, error) {
	addresses := []string{}
	var failure error
	for _, value := range raw {
		address, invalid := recipients.Addresses{}.Normalize(value)
		addresses = append(addresses, address)
		failure = cmp.Or(failure, invalid)
	}
	return addresses, failure
}

func (service *RecipientService) refusal(count int, invalid error) error {
	switch {
	case count > service.maxAddresses():
		return &InvalidRequest{Field: service.addressesField(), Message: "at most 100 addresses can be resolved at once"}
	case invalid != nil:
		return &InvalidRequest{Field: service.addressesField(), Message: invalid.Error()}
	}
	return nil
}

func (service *RecipientService) validOnly(refusal error, addresses []string) []string {
	if refusal != nil {
		return []string{}
	}
	return addresses
}

func (service *RecipientService) resolveAll(callerDomainId string, addresses []string, hosted map[string]bool) ([]ResolvedRecipient, error) {
	resolved := []ResolvedRecipient{}
	var failure error
	for _, address := range addresses {
		recipient, lookupFailure := service.lookup.Resolve(callerDomainId, address, hosted)
		resolved = append(resolved, recipient)
		failure = cmp.Or(failure, lookupFailure)
	}
	return resolved, failure
}

func (service *RecipientService) addressesField() string {
	return "addresses"
}

func (service *RecipientService) maxAddresses() int {
	return 100
}
