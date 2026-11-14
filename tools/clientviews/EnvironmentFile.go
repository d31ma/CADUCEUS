package main

import (
	"regexp"
	"strings"
)

type EnvironmentFile struct {
	Name   string
	Source string
}

func (file EnvironmentFile) Problems() []Problem {
	return file.linesProblems(strings.Split(file.Source, "\n"))
}

func (file EnvironmentFile) linesProblems(lines []string) []Problem {
	problems := []Problem{}
	for index, line := range lines {
		problems = append(problems, file.lineProblem(line, index+1)...)
	}
	return problems
}

func (file EnvironmentFile) lineProblem(line string, number int) []Problem {
	if !file.turnsCheckOff(line) {
		return nil
	}
	return []Problem{{File: file.Name, Line: number, Reason: file.turnedOffReason(line)}}
}

func (file EnvironmentFile) turnsCheckOff(line string) bool {
	return file.offPattern().MatchString(line)
}

func (EnvironmentFile) offPattern() *regexp.Regexp {
	return regexp.MustCompile(`^\s*(?:export\s+)?(?:TAC|YON)_DEPENDENCIES_CHECK\s*=\s*["']?(?i:false|0|off|no)["']?\s*(?:#.*)?$`)
}

func (EnvironmentFile) turnedOffReason(line string) string {
	return "turns off TACHYON's dependency check (" + strings.TrimSpace(line) + ", TY1327); remove the line"
}
