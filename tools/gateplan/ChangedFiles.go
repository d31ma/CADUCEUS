package main

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

type ChangedFiles struct {
	Root string
	Base string
}

func (files ChangedFiles) List() ChangeList {
	paths, failure := files.collect(files.commands())
	return ChangeList{Paths: slices.Compact(slices.Sorted(slices.Values(paths))), Failure: failure, Base: files.Base}
}

func (files ChangedFiles) commands() [][]string {
	return [][]string{
		{"diff", "--name-only", "--no-renames", "-z", files.Base + "...HEAD"},
		{"diff", "--name-only", "--no-renames", "-z", "HEAD"},
		{"ls-files", "--others", "--exclude-standard", "-z"},
	}
}

func (files ChangedFiles) collect(commands [][]string) ([]string, error) {
	paths := []string{}
	var failures error
	for _, arguments := range commands {
		found, failure := files.git(arguments)
		paths = append(paths, found...)
		failures = errors.Join(failures, failure)
	}
	return paths, failures
}

func (files ChangedFiles) git(arguments []string) ([]string, error) {
	output, failure := exec.Command("git", append([]string{"-C", files.Root}, arguments...)...).Output()
	return files.names(string(output)), files.described(arguments, failure)
}

func (ChangedFiles) names(output string) []string {
	return strings.FieldsFunc(output, func(letter rune) bool { return letter == 0 })
}

func (ChangedFiles) described(arguments []string, failure error) error {
	if failure == nil {
		return nil
	}
	return fmt.Errorf("git %s: %w", strings.Join(arguments, " "), failure)
}
