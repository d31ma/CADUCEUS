package messaging

type UnknownReaction struct {
	Name string
}

func (unknown UnknownReaction) Error() string {
	return "unknown reaction name: " + unknown.Name
}
