// FlowUtils names the two guards that settings actions share, so an action
// can say "only when" or "this, otherwise that" without branching inline.
type FlowUtils struct{}

// When runs action only when condition holds.
func (FlowUtils) When(condition bool, action func()) {
	FlowUtils{}.when(condition, action)
}

// Choose runs chosen when condition holds and otherwise the other action.
func (FlowUtils) Choose(condition bool, chosen func(), otherwise func()) {
	FlowUtils{}.choose(condition, chosen, otherwise)
}

// Both is true when both conditions hold.
func (FlowUtils) Both(first bool, second bool) bool {
	return FlowUtils{}.both(first, second)
}

// Either is true when at least one condition holds.
func (FlowUtils) Either(first bool, second bool) bool {
	return FlowUtils{}.either(first, second)
}

// Text answers chosen when condition holds and otherwise the other text.
func (FlowUtils) Text(condition bool, chosen string, otherwise string) string {
	return FlowUtils{}.text(condition, chosen, otherwise)
}

func (FlowUtils) both(first bool, second bool) bool {
	return first && second
}

func (FlowUtils) either(first bool, second bool) bool {
	return first || second
}

func (FlowUtils) text(condition bool, chosen string, otherwise string) string {
	if condition {
		return chosen
	}
	return otherwise
}

func (FlowUtils) when(condition bool, action func()) {
	if condition {
		action()
	}
}

func (FlowUtils) choose(condition bool, chosen func(), otherwise func()) {
	if condition {
		chosen()
		return
	}
	otherwise()
}
