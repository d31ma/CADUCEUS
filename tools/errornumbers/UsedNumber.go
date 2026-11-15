package main

type UsedNumber struct {
	Number int
	File   string
	Line   int
}

func (UsedNumber) Before(first UsedNumber, second UsedNumber) bool {
	if first.File != second.File {
		return first.File < second.File
	}
	return first.Line < second.Line
}
