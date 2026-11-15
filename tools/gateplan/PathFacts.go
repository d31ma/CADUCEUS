package main

type PathFacts struct {
	Path           string
	RunsEverything bool
	ModuleFile     bool
	InApp          bool
	OwnApps        []string
	KitPackage     string
	KitApps        []string
	Responsive     bool
	WebApps        []string
	Docs           bool
	Shared         bool
}

func (facts PathFacts) reach() Reach {
	switch {
	case facts.RunsEverything, facts.ModuleFile:
		return Reach{Everything: true, Reason: facts.Path + " changed"}
	case facts.InApp:
		return Reach{Apps: facts.OwnApps, Reason: "own files: " + facts.Path}
	case facts.KitPackage != "":
		return Reach{Apps: facts.KitApps, Reason: "uses kit/" + facts.KitPackage + ": " + facts.Path}
	case facts.Responsive:
		return Reach{Apps: facts.WebApps, Reason: "tests/responsive checks it: " + facts.Path}
	case facts.Docs:
		return Reach{Reason: facts.Path + " is documentation"}
	case facts.Shared:
		return Reach{Reason: facts.Path + " is checked by the shared steps"}
	}
	return Reach{Everything: true, Reason: facts.Path + " is outside every path the gate plan knows"}
}
