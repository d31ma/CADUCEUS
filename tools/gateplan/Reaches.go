package main

import (
	"fmt"
	"maps"
	"slices"
)

type Reaches []Reach

func (reaches Reaches) everythingReason() string {
	for _, reach := range reaches {
		if reach.Everything {
			return reach.Reason
		}
	}
	return ""
}

func (reaches Reaches) chosen() []ChosenApp {
	reasons := map[string]string{}
	for _, reach := range reaches {
		reach.addTo(reasons)
	}
	return reaches.sorted(reasons)
}

func (Reaches) sorted(reasons map[string]string) []ChosenApp {
	apps := []ChosenApp{}
	for _, name := range slices.Sorted(maps.Keys(reasons)) {
		apps = append(apps, ChosenApp{Name: name, Reason: reasons[name]})
	}
	return apps
}

func (reaches Reaches) noneReason() string {
	return fmt.Sprintf("no app uses the %d changed files: docs and shared code only", len(reaches))
}
