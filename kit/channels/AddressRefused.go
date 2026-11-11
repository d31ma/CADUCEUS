package channels

type AddressRefused struct {
	Address string
}

func (failure AddressRefused) Error() string {
	return "the address " + failure.Address + " is not in the channel's domain"
}
