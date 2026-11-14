package main

import "fmt"

type Problem struct {
	File string
	Line int
	Text string
}

func (Problem) NotListed(used UsedNumber) Problem {
	return Problem{File: used.File, Line: used.Line, Text: fmt.Sprintf("MS%04d is used in code but is not in the Numbers table of %s", used.Number, Problem{}.errorsFile())}
}

func (Problem) NotUsed(listed ListedNumber) Problem {
	return Problem{File: Problem{}.errorsFile(), Line: listed.Line, Text: fmt.Sprintf("MS%04d (%s) is listed but no code of %s uses it", listed.Number, listed.App, listed.App)}
}

func (Problem) UsedOutsideBlock(used UsedNumber, owner Owner) Problem {
	return Problem{File: used.File, Line: used.Line, Text: fmt.Sprintf("MS%04d is outside the block of %s", used.Number, owner.Block())}
}

func (Problem) ListedOutsideBlock(listed ListedNumber) Problem {
	return Problem{File: Problem{}.errorsFile(), Line: listed.Line, Text: fmt.Sprintf("MS%04d is listed for %s but is outside the block of %s", listed.Number, listed.App, listed.App)}
}

func (Problem) ListedTwice(listed ListedNumber) Problem {
	return Problem{File: Problem{}.errorsFile(), Line: listed.Line, Text: fmt.Sprintf("MS%04d is listed more than once", listed.Number)}
}

func (Problem) UsedTwice(used UsedNumber, first UsedNumber) Problem {
	return Problem{File: used.File, Line: used.Line, Text: fmt.Sprintf("MS%04d is used twice; it is already used at %s:%d (one number per case)", used.Number, first.File, first.Line)}
}

func (Problem) NotSorted(listed ListedNumber, previous ListedNumber) Problem {
	return Problem{File: Problem{}.errorsFile(), Line: listed.Line, Text: fmt.Sprintf("MS%04d comes after MS%04d; sort the rows by number", listed.Number, previous.Number)}
}

func (problem Problem) String() string {
	return fmt.Sprintf("%s:%d: %s", problem.File, problem.Line, problem.Text)
}

func (Problem) errorsFile() string {
	return "docs/mesail/ERRORS.md"
}
