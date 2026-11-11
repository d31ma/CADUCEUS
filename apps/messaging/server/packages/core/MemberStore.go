package core

type MemberStore interface {
	Save(member Member) error
	ByMemberId(domainId string, memberId string) (Member, bool, error)
	ByLocalPart(domainId string, localPart string) (Member, bool, error)
}
