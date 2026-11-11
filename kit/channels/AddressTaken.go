package channels

type AddressTaken struct {
	Address string
}

func (failure AddressTaken) Error() string {
	return "the address " + failure.Address + " already belongs to a member, an alias or another channel"
}
