package main

import "cmp"

type Planner struct {
	Folders AppFolders
}

func (planner Planner) Plan(changes ChangeList) Plan {
	reaches := planner.reaches(changes.Paths)
	return Plan{
		Everything: cmp.Or(changes.failureReason(), changes.emptyReason(), reaches.everythingReason()),
		Apps:       reaches.chosen(),
		None:       reaches.noneReason(),
	}
}

func (planner Planner) reaches(paths []string) Reaches {
	reaches := Reaches{}
	for _, path := range paths {
		reaches = append(reaches, ChangedPath(path).Reach(planner.Folders))
	}
	return reaches
}
