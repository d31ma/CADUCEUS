package channels

type Directory interface {
	MemberInDomain(authorization string, memberId string) (bool, error)
	ClaimChannelAddress(authorization string, address string, channelId string) error
	ReleaseChannelAddress(authorization string, address string) error
}
