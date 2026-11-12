package main

import (
	"fmt"
	"io"
)

type Command struct {
	Output io.Writer
}

func (command Command) Run(appFolders []string) int {
	problems, err := ClientCheck{}.Apps(appFolders)
	return command.exitCode(problems, err)
}

func (command Command) exitCode(problems []Problem, err error) int {
	switch {
	case err != nil:
		fmt.Fprintf(command.Output, "clientviews: %v\n", err)
		return 2
	case len(problems) > 0:
		command.print(problems)
		return 1
	}
	fmt.Fprintln(command.Output, "clientviews: the dependency check is on and every view shows only service values")
	return 0
}

func (command Command) print(problems []Problem) {
	for _, problem := range problems {
		fmt.Fprintln(command.Output, problem.String())
	}
	fmt.Fprintf(command.Output, "clientviews: %d problems break SLICES rule 19 (client layers)\n", len(problems))
}
