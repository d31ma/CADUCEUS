package addresses

type NameKinds struct{}

func (NameKinds) Username() string  { return "username" }
func (NameKinds) LocalPart() string { return "local part" }
func (NameKinds) Domain() string    { return "domain" }
func (NameKinds) Address() string   { return "address" }
