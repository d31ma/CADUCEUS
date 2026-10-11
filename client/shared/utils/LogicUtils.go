import "slices"

// LogicUtils joins conditions without && and ||, which the cohesion rule
// counts as decisions. Both sides are always evaluated, so never guard a
// nil value with them.
type LogicUtils struct{}

// All is true when every condition holds.
func (LogicUtils) All(conditions ...bool) bool {
	return !slices.Contains(conditions, false)
}

// Any is true when at least one condition holds.
func (LogicUtils) Any(conditions ...bool) bool {
	return slices.Contains(conditions, true)
}
