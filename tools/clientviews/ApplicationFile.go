package main

import (
	"encoding/json"
	"fmt"
)

type ApplicationFile struct {
	Name   string
	Source []byte
}

func (file ApplicationFile) Problems() ([]Problem, error) {
	settings := ApplicationSettings{}
	err := json.Unmarshal(file.Source, &settings)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file.Name, err)
	}
	return DependencySwitch{Name: file.Name, Source: file.Source, Reason: file.turnedOffReason()}.Problems(settings.Dependencies), nil
}

func (ApplicationFile) turnedOffReason() string {
	return "sets dependencies.check to false, which turns off TACHYON's dependency check (TY1327) for Yon code; remove the setting"
}
