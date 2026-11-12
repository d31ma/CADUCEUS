package outbox

type subscriberUnion struct{}

func (union subscriberUnion) of(first []string, second []string) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, name := range append(append([]string{}, first...), second...) {
		names = union.added(names, seen, name)
	}
	return names
}

func (subscriberUnion) added(names []string, seen map[string]bool, name string) []string {
	if seen[name] {
		return names
	}
	seen[name] = true
	return append(names, name)
}
