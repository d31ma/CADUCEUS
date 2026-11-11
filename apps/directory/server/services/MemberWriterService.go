import (
	"cmp"
	"errors"
	"slices"

	"mesail.dev/packages/addresses"
	"mesail.dev/packages/outbox"
)

// @Service
type MemberWriterService struct {
	// @Needs
	members *MemberStorageService
	// @Needs
	addresses *AddressStorageService
	// @Needs
	events *EventService
	// @Needs
	relay *OutboxRelayService
}

func (writer *MemberWriterService) Join(person PersonDocument, domainId string, admin bool) (MemberDocument, error) {
	existing, found, failure := writer.members.Find(domainId, person.PersonId)
	member := writer.joined(existing, person, domainId, admin)
	saved, failure := writer.writeUnless(failure, found, member)
	address := AddressDocument{Address: member.PrimaryAddress, DomainId: domainId, Kind: writer.memberKind(), TargetId: person.PersonId}
	return saved, writer.addressUnless(failure, address)
}

func (writer *MemberWriterService) Update(member MemberDocument) (MemberDocument, error) {
	return writer.write(true, member)
}

func (writer *MemberWriterService) Leave(member MemberDocument) error {
	member.State = writer.removed()
	member.Admin = false
	_, failure := writer.write(true, member)
	address, found, readFailure := writer.addresses.Find(member.PrimaryAddress)
	return writer.deleteOwn(cmp.Or(failure, readFailure), found, address, member.MemberId)
}

func (writer *MemberWriterService) leaveDomain(personId string, domainId string) error {
	member, found, failure := writer.members.Find(domainId, personId)
	return writer.leaveFound(found, failure, member)
}

func (writer *MemberWriterService) LeaveAll(person PersonDocument) error {
	return writer.leaveEach(person)
}

func (writer *MemberWriterService) mirror(person PersonDocument, domainId string) error {
	member, found, failure := writer.members.Find(domainId, person.PersonId)
	return writer.mirrorFound(found, failure, writer.mirrored(member, person))
}

func (writer *MemberWriterService) MirrorAll(person PersonDocument) error {
	return writer.mirrorEach(person)
}

func (writer *MemberWriterService) Active(domainId string, memberId string) (MemberDocument, error) {
	member, found, failure := writer.members.Find(domainId, memberId)
	return member, cmp.Or(failure, writer.present(found, member.State))
}

func (writer *MemberWriterService) OtherAdmin(member MemberDocument) error {
	members, failure := writer.members.All(member.DomainId)
	return cmp.Or(failure, writer.anotherAdmin(writer.hasOtherAdmin(members, member.MemberId)))
}

func (writer *MemberWriterService) write(found bool, member MemberDocument) (MemberDocument, error) {
	staged, stageFailure := writer.events.StageMemberSaved(member)
	ref, failure := writer.storeUnless(stageFailure, found, staged)
	writer.relay.Drain(member.DomainId, ref)
	staged.Id = ref.Id
	return staged, failure
}

func (writer *MemberWriterService) writeUnless(failure error, found bool, member MemberDocument) (MemberDocument, error) {
	if failure != nil {
		return MemberDocument{}, failure
	}
	return writer.write(found, member)
}

func (writer *MemberWriterService) storeUnless(failure error, found bool, member MemberDocument) (outbox.Ref, error) {
	switch {
	case failure != nil:
		return outbox.Ref{}, failure
	case found:
		return writer.members.Save(member)
	}
	return writer.members.Insert(member)
}

func (writer *MemberWriterService) addressUnless(failure error, address AddressDocument) error {
	if failure != nil {
		return failure
	}
	return writer.addresses.InsertIfAbsent(address)
}

func (writer *MemberWriterService) leaveFound(found bool, failure error, member MemberDocument) error {
	switch {
	case failure != nil:
		return failure
	case !found, member.State == writer.removed():
		return nil
	}
	return writer.Leave(member)
}

func (writer *MemberWriterService) leaveEach(person PersonDocument) error {
	failures := []error{}
	for _, domainId := range person.DomainIds {
		failures = append(failures, writer.leaveDomain(person.PersonId, domainId))
	}
	return errors.Join(failures...)
}

func (writer *MemberWriterService) mirrorEach(person PersonDocument) error {
	failures := []error{}
	for _, domainId := range person.DomainIds {
		failures = append(failures, writer.mirror(person, domainId))
	}
	return errors.Join(failures...)
}

func (writer *MemberWriterService) mirrorFound(found bool, failure error, member MemberDocument) error {
	switch {
	case failure != nil:
		return failure
	case !found:
		return nil
	}
	_, saveFailure := writer.write(true, member)
	return saveFailure
}

func (writer *MemberWriterService) deleteOwn(failure error, found bool, address AddressDocument, memberId string) error {
	switch {
	case failure != nil:
		return failure
	case !found, address.Kind != writer.memberKind(), address.TargetId != memberId:
		return nil
	}
	return writer.addresses.Delete(address)
}

func (writer *MemberWriterService) joined(existing MemberDocument, person PersonDocument, domainId string, admin bool) MemberDocument {
	member := existing
	member.MemberId = person.PersonId
	member.DomainId = domainId
	member.Username = person.Username
	member.DisplayName = person.DisplayName
	member.PrimaryAddress = addresses.Address{}.Primary(person.Username, domainId)
	member.Admin = admin
	member.State = writer.stateOf(person)
	return member
}

func (writer *MemberWriterService) mirrored(member MemberDocument, person PersonDocument) MemberDocument {
	member.DisplayName = person.DisplayName
	member.State = writer.mirroredState(member.State, writer.stateOf(person))
	return member
}

func (writer *MemberWriterService) mirroredState(current string, personState string) string {
	if current == writer.removed() {
		return current
	}
	return personState
}

func (writer *MemberWriterService) stateOf(person PersonDocument) string {
	if person.State == writer.disabled() {
		return writer.disabled()
	}
	return writer.active()
}

func (writer *MemberWriterService) present(found bool, state string) error {
	switch {
	case !found, state == writer.removed():
		return &NotFound{Message: "no such member in your domain"}
	}
	return nil
}

func (writer *MemberWriterService) hasOtherAdmin(members []MemberDocument, memberId string) bool {
	return slices.ContainsFunc(members, func(other MemberDocument) bool { return writer.adminBesides(other, memberId) })
}

func (writer *MemberWriterService) adminBesides(other MemberDocument, memberId string) bool {
	switch {
	case !other.Admin, other.State != writer.active(), other.MemberId == memberId:
		return false
	}
	return true
}

func (writer *MemberWriterService) anotherAdmin(found bool) error {
	if found {
		return nil
	}
	return &LastAdmin{Message: "a domain must keep at least one admin"}
}

func (writer *MemberWriterService) memberKind() string {
	return "member"
}

func (writer *MemberWriterService) active() string {
	return "active"
}

func (writer *MemberWriterService) disabled() string {
	return "disabled"
}

func (writer *MemberWriterService) removed() string {
	return "removed"
}
