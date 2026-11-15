package main

import "cmp"

type Reach struct {
	Everything bool
	Apps       []string
	Reason     string
}

func (reach Reach) addTo(reasons map[string]string) {
	for _, app := range reach.Apps {
		reasons[app] = cmp.Or(reasons[app], reach.Reason)
	}
}
