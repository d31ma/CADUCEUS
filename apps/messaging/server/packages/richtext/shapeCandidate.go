package richtext

type shapeCandidate struct {
	active bool
	shape  Block
}

func (candidate shapeCandidate) isActive() bool {
	return candidate.active
}
