import (
	"cmp"

	"mesail.dev/packages/oidc"
)

// @Service
type AliasService struct {
	// @Needs
	addresses *AddressStorageService
	// @Needs
	members *MemberStorageService
	// @Needs
	rules *AddressRulesService
	// @Needs
	reservations *ReservationService
}

func (service *AliasService) Add(admission Admission, request CreateAliasRequest) (Address, error) {
	return service.addUnless(admission, request)
}

func (service *AliasService) addUnless(admission Admission, request CreateAliasRequest) (Address, error) {
	if admission.Refusal != nil {
		return Address{}, admission.Refusal
	}
	return service.add(admission.Caller, request)
}

func (service *AliasService) add(caller oidc.Caller, request CreateAliasRequest) (Address, error) {
	address, invalid := service.rules.OwnAddress(caller.DomainId, request.Address)
	memberId := cmp.Or(request.MemberId, caller.MemberId)
	_, found, memberFailure := service.members.Find(caller.DomainId, memberId)
	_, taken, addressFailure := service.addresses.Find(address)
	alias := AddressDocument{Address: address, DomainId: caller.DomainId, Kind: service.aliasKind(), TargetId: memberId}
	failure := cmp.Or(invalid, memberFailure, addressFailure, service.knownMember(found), service.free(taken, address))
	failure = Steps{}.Unless(failure, func() error { return service.reservations.ForAddress(alias.Address) })
	return service.wire(alias), service.insertUnless(failure, alias)
}

func (service *AliasService) knownMember(found bool) error {
	if found {
		return nil
	}
	return &NotFound{Message: "no such member in your domain"}
}

func (service *AliasService) free(taken bool, address string) error {
	if taken {
		return &Duplicate{Message: address + " is already in use"}
	}
	return nil
}

func (service *AliasService) insertUnless(failure error, alias AddressDocument) error {
	if failure != nil {
		return failure
	}
	return service.addresses.Insert(alias)
}

func (service *AliasService) wire(alias AddressDocument) Address {
	return Address{Address: alias.Address, DomainId: alias.DomainId, Kind: alias.Kind, TargetId: alias.TargetId}
}

func (service *AliasService) aliasKind() string {
	return "alias"
}
