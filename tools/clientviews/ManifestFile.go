package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type ManifestFile struct {
	Name   string
	Source []byte
}

func (file ManifestFile) Problems() ([]Problem, error) {
	settings := ManifestSettings{}
	err := json.Unmarshal(file.Source, &settings)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file.Name, err)
	}
	return file.dependencyProblems(settings.Tac.Dependencies), nil
}

func (file ManifestFile) dependencyProblems(dependencies DependencySettings) []Problem {
	if !dependencies.TurnedOff() {
		return []Problem{}
	}
	return []Problem{{File: file.Name, Line: file.dependenciesLine(), Reason: file.turnedOffReason()}}
}

func (file ManifestFile) dependenciesLine() int {
	start := max(bytes.Index(file.Source, []byte(`"dependencies"`)), 0)
	return bytes.Count(file.Source[:start], []byte("\n")) + 1
}

func (ManifestFile) turnedOffReason() string {
	return "sets tac.dependencies.check to false, which turns off TACHYON's dependency check (TY1327); remove the setting"
}
