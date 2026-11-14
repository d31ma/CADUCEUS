package main

import "bytes"

type DependencySwitch struct {
	Name   string
	Source []byte
	Reason string
}

func (setting DependencySwitch) Problems(dependencies DependencySettings) []Problem {
	if !dependencies.TurnedOff() {
		return []Problem{}
	}
	return []Problem{{File: setting.Name, Line: setting.line(), Reason: setting.Reason}}
}

func (setting DependencySwitch) line() int {
	start := max(bytes.Index(setting.Source, []byte(`"dependencies"`)), 0)
	return bytes.Count(setting.Source[:start], []byte("\n")) + 1
}
