package main

import "fmt"

type Problem struct {
	File   string
	Line   int
	Reason string
}

func (problem Problem) String() string {
	return fmt.Sprintf("%s:%d: %s", problem.File, problem.Line, problem.Reason)
}
