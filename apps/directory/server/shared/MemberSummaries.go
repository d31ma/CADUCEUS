type MemberSummaries struct{}

func (MemberSummaries) Plain(member MemberDocument) MemberSummary {
	return MemberSummary{MemberId: member.MemberId, DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress}
}

func (MemberSummaries) Admin(member MemberDocument, accountManaged bool) MemberSummary {
	return MemberSummary{MemberId: member.MemberId, Username: member.Username, DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress, Admin: member.Admin, AccountManaged: accountManaged}
}
