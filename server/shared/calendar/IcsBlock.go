// IcsBlock is one top-level VEVENT of a resource's unfolded lines: the
// indexes of its BEGIN and END lines and the event it parses to (nil when
// it is incomplete).
type IcsBlock struct {
	Start int
	End   int
	Event map[string]any
}
