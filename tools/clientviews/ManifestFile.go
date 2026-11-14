package main

import (
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
	return DependencySwitch{Name: file.Name, Source: file.Source, Reason: file.turnedOffReason()}.Problems(settings.Tac.Dependencies), nil
}

func (ManifestFile) turnedOffReason() string {
	return "sets tac.dependencies.check to false, which turns off TACHYON's dependency check (TY1327); remove the setting"
}
