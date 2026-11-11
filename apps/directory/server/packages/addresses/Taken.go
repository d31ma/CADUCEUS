package addresses

type Taken struct {
	LocalPart string
}

func (taken Taken) Error() string {
	return "the local part " + taken.LocalPart + " is already in use"
}
