import "mesail.dev/packages/recipients"

// @Service
type RecipientLookupService struct {
	// @Needs
	targets *LocalTargetService
}

func (service *RecipientLookupService) Resolve(callerDomainId string, address string, hosted map[string]bool) (ResolvedRecipient, error) {
	domainId := recipients.Addresses{}.Domain(address)
	target, failure := service.local(callerDomainId == domainId, domainId, address)
	kind := recipients.Classifier{}.Kind(recipients.Destination{CallerDomainId: callerDomainId, AddressDomainId: domainId, Hosted: hosted[domainId], LocalKind: recipients.Kind(target.Kind)})
	return service.recipient(address, kind, target), failure
}

func (service *RecipientLookupService) local(sameDomain bool, domainId string, address string) (LocalTarget, error) {
	if !sameDomain {
		return LocalTarget{}, nil
	}
	return service.targets.Find(domainId, address)
}

func (service *RecipientLookupService) recipient(address string, kind recipients.Kind, target LocalTarget) ResolvedRecipient {
	policy := recipients.Policy{}
	return ResolvedRecipient{
		Address:           address,
		DisplayName:       target.DisplayName,
		Kind:              string(kind),
		MemberId:          target.MemberId,
		ChannelId:         target.ChannelId,
		DefaultDelivery:   string(policy.Default(kind)),
		AllowedDeliveries: service.names(policy.Allowed(kind)),
	}
}

func (service *RecipientLookupService) names(deliveries []recipients.Delivery) []string {
	names := []string{}
	for _, delivery := range deliveries {
		names = append(names, string(delivery))
	}
	return names
}
