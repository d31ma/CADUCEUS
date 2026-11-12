package main

import (
	"io/fs"
	"os"
	"path/filepath"
)

type ClientCheck struct{}

func (check ClientCheck) Apps(appFolders []string) ([]Problem, error) {
	return check.appsProblems(appFolders)
}

func (check ClientCheck) appsProblems(appFolders []string) ([]Problem, error) {
	problems := []Problem{}
	for _, folder := range appFolders {
		found, err := check.appProblems(filepath.Join(folder, "client"))
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func (check ClientCheck) appProblems(client string) ([]Problem, error) {
	manifestProblems, err := check.manifestProblems(filepath.Join(client, "manifest.json"))
	if err != nil {
		return nil, err
	}
	viewProblems, err := check.viewProblems(client)
	if err != nil {
		return nil, err
	}
	return append(manifestProblems, viewProblems...), nil
}

func (ClientCheck) manifestProblems(name string) ([]Problem, error) {
	source, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return ManifestFile{Name: name, Source: source}.Problems()
}

func (check ClientCheck) viewProblems(client string) ([]Problem, error) {
	names, err := check.viewFiles(client)
	if err != nil {
		return nil, err
	}
	return check.problemsInViews(names)
}

func (check ClientCheck) viewFiles(client string) ([]string, error) {
	names := []string{}
	err := filepath.WalkDir(client, func(name string, entry fs.DirEntry, walkError error) error {
		names = append(names, check.viewName(name, entry)...)
		return walkError
	})
	return names, err
}

func (ClientCheck) viewName(name string, entry fs.DirEntry) []string {
	if entry == nil || entry.IsDir() || entry.Name() != "tac.html" {
		return nil
	}
	return []string{name}
}

func (check ClientCheck) problemsInViews(names []string) ([]Problem, error) {
	problems := []Problem{}
	for _, name := range names {
		found, err := check.viewFileProblems(name)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func (ClientCheck) viewFileProblems(name string) ([]Problem, error) {
	source, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return ViewFile{Name: name, Source: string(source)}.Problems(), nil
}
