package main

import (
	"fmt"
	"io"
)

type Command struct {
	Output io.Writer
}

func (command Command) Run(appFolders []string) int {
	problems, err := ControllerCheck{}.Apps(appFolders)
	return command.exitCode(problems, err)
}

func (command Command) exitCode(problems []Problem, err error) int {
	switch {
	case err != nil:
		fmt.Fprintf(command.Output, "controllers: %v\n", err)
		return 2
	case len(problems) > 0:
		command.print(problems)
		return 1
	}
	fmt.Fprintln(command.Output, "controllers: every route method returns one Service call")
	return 0
}

func (command Command) print(problems []Problem) {
	for _, problem := range problems {
		fmt.Fprintln(command.Output, problem.String())
	}
	fmt.Fprintf(command.Output, "controllers: %d route methods break ADR 0009; write each as: return controller.<service>.<Method>(<parameters>)\n", len(problems))
}
