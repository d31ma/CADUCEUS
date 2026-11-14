package realtime

type DeliverySplit struct {
	Reached []string
	Missed  []string
}

func (split DeliverySplit) With(key string, reached bool) DeliverySplit {
	if reached {
		split.Reached = append(split.Reached, key)
		return split
	}
	split.Missed = append(split.Missed, key)
	return split
}
