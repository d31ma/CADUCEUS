import "cmp"

// @Service
type LocalTargetService struct {
	// @Needs
	addresses *AddressStorageService
	// @Needs
	members *MemberStorageService
}

func (service *LocalTargetService) Find(domainId string, address string) (LocalTarget, error) {
	route, found, failure := service.addresses.Find(address)
	member, memberFailure := service.memberUnless(failure, found, route)
	return service.target(route, member), cmp.Or(failure, memberFailure)
}

func (service *LocalTargetService) memberUnless(failure error, found bool, route AddressDocument) (MemberDocument, error) {
	switch {
	case failure != nil, !found, route.Kind == service.channelKind():
		return MemberDocument{}, nil
	}
	member, _, memberFailure := service.members.Find(route.DomainId, route.TargetId)
	return member, memberFailure
}

func (service *LocalTargetService) target(route AddressDocument, member MemberDocument) LocalTarget {
	switch {
	case route.Kind == service.channelKind():
		return LocalTarget{Kind: service.channelKind(), ChannelId: route.TargetId}
	case member.State == service.active():
		return LocalTarget{Kind: service.memberKind(), MemberId: member.MemberId, DisplayName: member.DisplayName}
	}
	return LocalTarget{}
}

func (service *LocalTargetService) channelKind() string {
	return "channel"
}

func (service *LocalTargetService) memberKind() string {
	return "member"
}

func (service *LocalTargetService) active() string {
	return "active"
}
