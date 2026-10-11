// FlowUtils picks the next step of a service method, so that the method that
// makes a decision only returns the step and the coordinating method runs it.
type FlowUtils struct{}

// When answers next when the condition holds, and a step that does nothing
// otherwise.
func (FlowUtils) When(condition bool, next func()) func() {
	return FlowUtils{}.either(condition, next, func() {})
}

// Either answers chosen when the condition holds, and other otherwise.
func (FlowUtils) Either(condition bool, chosen func(), other func()) func() {
	return FlowUtils{}.either(condition, chosen, other)
}

func (FlowUtils) either(condition bool, chosen func(), other func()) func() {
	if condition {
		return chosen
	}
	return other
}
