package channels

type MemoryDirectory struct {
	members  map[string]bool
	taken    map[string]bool
	claimed  map[string]string
	released []string
	failure  error
}

func NewMemoryDirectory() *MemoryDirectory {
	return &MemoryDirectory{members: map[string]bool{}, taken: map[string]bool{}, claimed: map[string]string{}, released: []string{}}
}

func (fake *MemoryDirectory) Knows(memberIds ...string) {
	for _, memberId := range memberIds {
		fake.members[memberId] = true
	}
}

func (fake *MemoryDirectory) Take(address string) {
	fake.taken[address] = true
}

func (fake *MemoryDirectory) Fail(failure error) {
	fake.failure = failure
}

func (fake *MemoryDirectory) ClaimedBy(address string) string {
	return fake.claimed[address]
}

func (fake *MemoryDirectory) Released() []string {
	return fake.released
}

func (fake *MemoryDirectory) MemberInDomain(authorization string, memberId string) (bool, error) {
	return fake.members[memberId], fake.failure
}

func (fake *MemoryDirectory) ClaimChannelAddress(authorization string, address string, channelId string) error {
	if fake.failure != nil {
		return fake.failure
	}
	if fake.taken[address] {
		return AddressTaken{Address: address}
	}
	fake.claimed[address] = channelId
	return nil
}

func (fake *MemoryDirectory) ReleaseChannelAddress(authorization string, address string) error {
	fake.released = append(fake.released, address)
	delete(fake.claimed, address)
	return fake.failure
}
