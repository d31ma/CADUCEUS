import (
	"cmp"
	"strings"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/recipients"
)

// @Service
type TokenMemberService struct {
	// @Needs
	tenants *TenantService
	// @Needs
	members *MemberStorageService
	// @Needs
	addresses *AddressStorageService
	// @Needs
	events *EventService
	// @Needs
	relay *OutboxRelayService
}

func (service *TokenMemberService) Remember(caller oidc.Caller) (MemberDocument, error) {
	_, refused := service.tenants.Require(caller.DomainId)
	existing, found, readFailure := service.findUnless(refused, caller)
	wanted := service.fromToken(caller, existing)
	staged, stageFailure := service.events.StageMemberSaved(wanted)
	changed := service.facts(existing) != service.facts(wanted)
	ref, failure := service.storeUnless(cmp.Or(refused, readFailure, stageFailure), found, changed, staged)
	service.relay.Drain(caller.DomainId, ref)
	return staged, service.addressUnless(failure, wanted)
}

func (service *TokenMemberService) findUnless(refused error, caller oidc.Caller) (MemberDocument, bool, error) {
	if refused != nil {
		return MemberDocument{}, false, nil
	}
	return service.members.Find(caller.DomainId, caller.MemberId)
}

func (service *TokenMemberService) storeUnless(failure error, found bool, changed bool, member MemberDocument) (outbox.Ref, error) {
	switch {
	case failure != nil:
		return outbox.Ref{}, failure
	case !found:
		return service.members.Insert(member)
	case changed:
		return service.members.Save(member)
	}
	return outbox.Ref{}, nil
}

func (service *TokenMemberService) addressUnless(failure error, member MemberDocument) error {
	switch {
	case failure != nil:
		return failure
	case member.PrimaryAddress == "":
		return nil
	}
	return service.addresses.InsertIfAbsent(AddressDocument{Address: member.PrimaryAddress, DomainId: member.DomainId, Kind: service.memberKind(), TargetId: member.MemberId})
}

func (service *TokenMemberService) fromToken(caller oidc.Caller, existing MemberDocument) MemberDocument {
	member := existing
	member.MemberId = caller.MemberId
	member.DomainId = caller.DomainId
	member.DisplayName = cmp.Or(strings.TrimSpace(caller.DisplayName), caller.Email)
	member.PrimaryAddress = service.ownAddress(caller)
	member.State = service.active()
	return member
}

func (service *TokenMemberService) facts(member MemberDocument) MemberFacts {
	return MemberFacts{DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress, Admin: member.Admin, State: member.State}
}

func (service *TokenMemberService) ownAddress(caller oidc.Caller) string {
	address, invalid := recipients.Addresses{}.Normalize(caller.Email)
	switch {
	case invalid != nil:
		return ""
	case (recipients.Addresses{}).Domain(address) != caller.DomainId:
		return ""
	}
	return address
}

func (service *TokenMemberService) active() string {
	return "active"
}

func (service *TokenMemberService) memberKind() string {
	return "member"
}
