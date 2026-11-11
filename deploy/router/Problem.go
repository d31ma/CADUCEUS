package main

type Problem struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (problem Problem) Error() string {
	return problem.Code + ": " + problem.Message
}
