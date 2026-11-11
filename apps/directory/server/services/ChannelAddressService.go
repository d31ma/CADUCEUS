import "cmp"

// @Service
type ChannelAddressService struct {
	// @Needs
	addresses *AddressStorageService
	// @Needs
	rules *AddressRulesService
	// @Needs
	reservations *ReservationService
}

func (service *ChannelAddressService) Add(admission Admission, request ChannelAddressRequest) (Address, error) {
	address, invalid := service.rules.OwnAddress(admission.Caller.DomainId, request.Address)
	existing, found, readFailure := service.addresses.Find(address)
	channel := AddressDocument{Address: address, DomainId: admission.Caller.DomainId, Kind: service.channelKind(), TargetId: request.ChannelId}
	failure := cmp.Or(admission.Refusal, invalid, readFailure, service.channelNamed(request.ChannelId))
	failure = Steps{}.Unless(failure, func() error { return service.reservations.ForAddress(channel.Address) })
	return service.wire(channel), service.storeUnless(failure, found, existing, channel)
}

func (service *ChannelAddressService) Remove(admission Admission, raw string) error {
	address, _ := service.rules.OwnAddress(admission.Caller.DomainId, raw)
	existing, found, readFailure := service.addresses.Find(address)
	failure := service.deleteUnless(cmp.Or(admission.Refusal, readFailure, service.channelOnly(found, existing.Kind)), existing)
	return Steps{}.Unless(failure, func() error { return service.reservations.Release(existing.Address) })
}

func (service *ChannelAddressService) storeUnless(failure error, found bool, existing AddressDocument, channel AddressDocument) error {
	switch {
	case failure != nil:
		return failure
	case service.same(existing, channel):
		return nil
	case found:
		return &Duplicate{Message: channel.Address + " is already in use"}
	}
	return service.addresses.Insert(channel)
}

func (service *ChannelAddressService) deleteUnless(failure error, existing AddressDocument) error {
	if failure != nil {
		return failure
	}
	return service.addresses.Delete(existing)
}

func (service *ChannelAddressService) channelOnly(found bool, kind string) error {
	switch {
	case !found, kind != service.channelKind():
		return &NotFound{Message: "no such channel address"}
	}
	return nil
}

func (service *ChannelAddressService) channelNamed(channelId string) error {
	if channelId == "" {
		return &InvalidRequest{Field: "channelId", Message: "a channel id is required"}
	}
	return nil
}

func (service *ChannelAddressService) same(existing AddressDocument, channel AddressDocument) bool {
	existing.Id = ""
	return existing == channel
}

func (service *ChannelAddressService) wire(channel AddressDocument) Address {
	return Address{Address: channel.Address, DomainId: channel.DomainId, Kind: channel.Kind, TargetId: channel.TargetId}
}

func (service *ChannelAddressService) channelKind() string {
	return "channel"
}
