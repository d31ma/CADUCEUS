package channels

type Commander interface {
	Command(command map[string]any) (any, error)
}
