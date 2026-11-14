package main

import (
	"fmt"
	"io"
)

type Command struct {
	Output io.Writer
}

func (command Command) Run(arguments []string) int {
	problems, err := NumberCheck{}.Repository(arguments)
	return command.exitCode(problems, err)
}

func (command Command) exitCode(problems []Problem, err error) int {
	switch {
	case err != nil:
		fmt.Fprintf(command.Output, "errornumbers: %v\n", err)
		return 2
	case len(problems) > 0:
		command.print(problems)
		return 1
	}
	fmt.Fprintln(command.Output, "errornumbers: every error number in code is listed once in docs/mesail/ERRORS.md, inside its app's block")
	return 0
}

func (command Command) print(problems []Problem) {
	for _, problem := range problems {
		fmt.Fprintln(command.Output, problem.String())
	}
	fmt.Fprintf(command.Output, "errornumbers: %d problems; docs/mesail/ERRORS.md and the code must list the same numbers (ADR 0009)\n", len(problems))
}
