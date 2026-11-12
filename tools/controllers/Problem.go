package main

import "fmt"

type Problem struct {
	File       string
	Line       int
	Controller string
	Method     string
	Reason     string
}

func (problem Problem) String() string {
	return fmt.Sprintf("%s:%d: %s.%s %s", problem.File, problem.Line, problem.Controller, problem.Method, problem.Reason)
}
