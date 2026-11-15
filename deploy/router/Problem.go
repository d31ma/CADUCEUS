package main

import "mesail.dev/packages/apierrors"

type Problem struct {
	Status      int    `json:"-"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	ErrorNumber int    `json:"-"`
}

func (problem Problem) Error() string {
	return problem.Code + ": " + problem.Message
}

func (problem Problem) Number() int {
	return problem.ErrorNumber
}

func (problem Problem) Numbered() Problem {
	problem.Message = apierrors.Message{}.Format(problem.ErrorNumber, problem.Message)
	return problem
}
