package main

type ChosenApp struct {
	Name   string
	Reason string
}

func (app ChosenApp) Line() string {
	return app.Name + "\t" + app.Reason + "\n"
}
