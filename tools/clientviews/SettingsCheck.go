package main

import "path/filepath"

type SettingsCheck struct{}

func (check SettingsCheck) App(folder string) ([]Problem, error) {
	applicationProblems, err := check.applicationProblems(filepath.Join(folder, "server", "application.json"))
	if err != nil {
		return nil, err
	}
	environmentProblems, err := check.environmentProblems(check.environmentNames(folder))
	if err != nil {
		return nil, err
	}
	return append(applicationProblems, environmentProblems...), nil
}

func (SettingsCheck) environmentNames(folder string) []string {
	return []string{filepath.Join(folder, ".env"), filepath.Join(folder, ".env.example")}
}

func (SettingsCheck) applicationProblems(name string) ([]Problem, error) {
	source, found, err := SettingFile{Name: name}.Read()
	if !found {
		return []Problem{}, err
	}
	return ApplicationFile{Name: name, Source: source}.Problems()
}

func (check SettingsCheck) environmentProblems(names []string) ([]Problem, error) {
	problems := []Problem{}
	for _, name := range names {
		found, err := check.environmentFileProblems(name)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func (SettingsCheck) environmentFileProblems(name string) ([]Problem, error) {
	source, _, err := SettingFile{Name: name}.Read()
	return EnvironmentFile{Name: name, Source: string(source)}.Problems(), err
}
