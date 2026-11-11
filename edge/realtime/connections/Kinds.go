package connections

type Kinds struct{}

func (Kinds) Socket() string { return "socket" }
func (Kinds) Stream() string { return "stream" }
