package main

type ListedNumber struct {
	Number int
	App    string
	Line   int
}

func (listed ListedNumber) SameNumber(other ListedNumber) bool {
	return listed.Number == other.Number
}
