package main

import "strings"

type ModuleFile struct {
	Text string
}

func (file ModuleFile) Path() string {
	paths := []string{}
	for _, line := range strings.Split(file.Text, "\n") {
		paths = append(paths, file.modulePath(strings.Fields(line))...)
	}
	return strings.Join(paths, "")
}

func (ModuleFile) modulePath(words []string) []string {
	if len(words) != 2 || words[0] != "module" {
		return nil
	}
	return []string{strings.Trim(words[1], "\"`")}
}
