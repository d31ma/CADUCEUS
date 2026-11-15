package main

import (
	"errors"
	"fmt"
	"io"
)

type Command struct {
	Output io.Writer
	Errors io.Writer
}

func (command Command) Run(values []string) int {
	arguments, argumentsFailure := Arguments{}.Parse(values)
	folders, foldersFailure := AppFolderReader{Root: arguments.Root}.Read()
	changes := ChangedFiles{Root: arguments.Root, Base: arguments.Base}.List()
	plan := Planner{Folders: folders}.Plan(changes)
	return command.answer(plan, errors.Join(argumentsFailure, foldersFailure))
}

func (command Command) answer(plan Plan, failure error) int {
	if failure != nil {
		fmt.Fprintf(command.Errors, "gateplan: %v\n", failure)
		return 2
	}
	fmt.Fprint(command.Output, plan.Text())
	return 0
}
