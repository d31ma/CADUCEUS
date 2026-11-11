import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/recipients"
)

// @Service
type MemberService struct {
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

func (service *MemberService) Remember(caller oidc.Caller) (MemberDocument, error) {
	_, refused := service.tenants.Require(caller.DomainId)
	existing, found, readFailure := service.findUnless(refused, caller)
	wanted := service.fromToken(caller, existing)
	staged, stageFailure := service.events.StageMemberSaved(wanted)
	changed := service.facts(existing) != service.facts(wanted)
	ref, failure := service.storeUnless(cmp.Or(refused, readFailure, stageFailure), found, changed, staged)
	service.relay.Drain(caller.DomainId, ref)
	return staged, service.addressUnless(failure, wanted)
}

func (service *MemberService) Search(admission Admission, query MemberQuery, invalid error) (MemberPage, error) {
	size, sizeInvalid := service.pageSize(query.Limit)
	offset, cursorInvalid := service.offset(query.Cursor)
	refusal := cmp.Or(admission.Refusal, invalid, sizeInvalid, cursorInvalid)
	members, failure := service.allUnless(refusal, admission.Caller.DomainId)
	return service.page(service.matching(members, query.Text), offset, size), cmp.Or(refusal, failure)
}

func (service *MemberService) Summary(admission Admission, memberId string) (MemberSummary, error) {
	member, found, failure := service.memberUnless(admission, memberId)
	summary := MemberSummary{MemberId: member.MemberId, DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress}
	return summary, cmp.Or(admission.Refusal, failure, service.knownActive(found, member.State))
}

func (service *MemberService) memberUnless(admission Admission, memberId string) (MemberDocument, bool, error) {
	if admission.Refusal != nil {
		return MemberDocument{}, false, nil
	}
	return service.members.Find(admission.Caller.DomainId, memberId)
}

func (service *MemberService) knownActive(found bool, state string) error {
	switch {
	case !found, state != service.active():
		return &NotFound{Message: "no such member in your domain"}
	}
	return nil
}

func (service *MemberService) findUnless(refused error, caller oidc.Caller) (MemberDocument, bool, error) {
	if refused != nil {
		return MemberDocument{}, false, nil
	}
	return service.members.Find(caller.DomainId, caller.MemberId)
}

func (service *MemberService) storeUnless(failure error, found bool, changed bool, member MemberDocument) (outbox.Ref, error) {
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

func (service *MemberService) addressUnless(failure error, member MemberDocument) error {
	switch {
	case failure != nil:
		return failure
	case member.PrimaryAddress == "":
		return nil
	}
	return service.addresses.InsertIfAbsent(AddressDocument{Address: member.PrimaryAddress, DomainId: member.DomainId, Kind: service.memberKind(), TargetId: member.MemberId})
}

func (service *MemberService) allUnless(refusal error, domainId string) ([]MemberDocument, error) {
	if refusal != nil {
		return []MemberDocument{}, nil
	}
	return service.members.All(domainId)
}

func (service *MemberService) offset(cursor string) (int, error) {
	value, failure := strconv.Atoi(cursor)
	switch {
	case cursor == "":
		return 0, nil
	case failure != nil, value < 0:
		return 0, &InvalidRequest{Field: "cursor", Message: "the cursor is not one this service gave out"}
	}
	return value, nil
}

func (service *MemberService) page(members []MemberDocument, offset int, size int) MemberPage {
	start := min(offset, len(members))
	end := min(offset+size, len(members))
	return MemberPage{Items: service.summaries(members[start:end]), NextCursor: service.nextCursor(end, len(members))}
}

func (service *MemberService) nextCursor(end int, total int) string {
	if end >= total {
		return ""
	}
	return strconv.Itoa(end)
}

func (service *MemberService) fromToken(caller oidc.Caller, existing MemberDocument) MemberDocument {
	return MemberDocument{
		Id:             existing.Id,
		MemberId:       caller.MemberId,
		DomainId:       caller.DomainId,
		DisplayName:    cmp.Or(strings.TrimSpace(caller.DisplayName), caller.Email),
		PrimaryAddress: service.ownAddress(caller),
		State:          service.active(),
		PendingEvents:  existing.PendingEvents,
		DeadEvents:     existing.DeadEvents,
		OutboxPending:  existing.OutboxPending,
		OutboxDead:     existing.OutboxDead,
	}
}

func (service *MemberService) facts(member MemberDocument) MemberFacts {
	return MemberFacts{DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress, State: member.State}
}

func (service *MemberService) ownAddress(caller oidc.Caller) string {
	address, invalid := recipients.Addresses{}.Normalize(caller.Email)
	switch {
	case invalid != nil:
		return ""
	case (recipients.Addresses{}).Domain(address) != caller.DomainId:
		return ""
	}
	return address
}

func (service *MemberService) pageSize(limit int) (int, error) {
	switch {
	case limit == 0:
		return service.defaultLimit(), nil
	case limit < 1, limit > service.maxLimit():
		return 0, &InvalidRequest{Field: "limit", Message: "limit must be between 1 and 100"}
	}
	return limit, nil
}

func (service *MemberService) matching(members []MemberDocument, query string) []MemberDocument {
	wanted := strings.ToLower(strings.TrimSpace(query))
	return slices.DeleteFunc(members, func(member MemberDocument) bool {
		return !strings.Contains(strings.ToLower(member.DisplayName+" "+member.PrimaryAddress), wanted)
	})
}

func (service *MemberService) summaries(members []MemberDocument) []MemberSummary {
	summaries := []MemberSummary{}
	for _, member := range members {
		summaries = append(summaries, MemberSummary{MemberId: member.MemberId, DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress})
	}
	return summaries
}

func (service *MemberService) active() string {
	return "active"
}

func (service *MemberService) memberKind() string {
	return "member"
}

func (service *MemberService) defaultLimit() int {
	return 20
}

func (service *MemberService) maxLimit() int {
	return 100
}
