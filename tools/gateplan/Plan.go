package main

import "strings"

type Plan struct {
	Everything string
	Apps       []ChosenApp
	None       string
}

func (plan Plan) Text() string {
	return plan.text(plan.appLines())
}

func (plan Plan) text(appLines string) string {
	switch {
	case plan.Everything != "":
		return "*\t" + plan.oneLine(plan.Everything) + "\n"
	case appLines == "":
		return "-\t" + plan.None + "\n"
	}
	return appLines
}

func (plan Plan) appLines() string {
	lines := strings.Builder{}
	for _, app := range plan.Apps {
		lines.WriteString(app.Line())
	}
	return lines.String()
}

func (Plan) oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
