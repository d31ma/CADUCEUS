package main

import (
	"os"
	"path/filepath"
)

type ControllerCheck struct{}

func (check ControllerCheck) Apps(appFolders []string) ([]Problem, error) {
	files, err := check.controllerFiles(appFolders)
	if err != nil {
		return nil, err
	}
	return check.problems(files)
}

func (check ControllerCheck) controllerFiles(appFolders []string) ([]string, error) {
	files := []string{}
	for _, folder := range appFolders {
		found, err := check.appControllerFiles(folder)
		if err != nil {
			return nil, err
		}
		files = append(files, found...)
	}
	return files, nil
}

func (ControllerCheck) appControllerFiles(folder string) ([]string, error) {
	if _, err := os.Stat(folder); err != nil {
		return nil, err
	}
	return filepath.Glob(filepath.Join(folder, "server", "controllers", "*Controller.go"))
}

func (check ControllerCheck) problems(files []string) ([]Problem, error) {
	problems := []Problem{}
	for _, name := range files {
		found, err := check.fileProblems(name)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func (ControllerCheck) fileProblems(name string) ([]Problem, error) {
	source, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return SourceFile{Name: name, Source: source}.Problems()
}
