package addresses

type Owners struct{}

func (Owners) Person() string    { return "person" }
func (Owners) Addresses() string { return "addresses" }
