package main

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type ErrorsTable struct {
	Owners Owners
	Rows   []ListedNumber
}

func (table ErrorsTable) Parse(text string) ErrorsTable {
	lines := strings.Split(text, "\n")
	blocks := table.sectionStart(lines, "## Blocks")
	numbers := table.sectionStart(lines, "## Numbers")
	owners := table.owners(lines, blocks, table.sectionEnd(lines, blocks))
	return ErrorsTable{Owners: owners, Rows: table.rows(lines, numbers, table.sectionEnd(lines, numbers))}
}

func (table ErrorsTable) Problems() []Problem {
	problems := table.outsideBlockProblems()
	problems = append(problems, table.twiceProblems()...)
	return append(problems, table.orderProblems()...)
}

func (table ErrorsTable) Lists(number int) bool {
	return slices.ContainsFunc(table.Rows, ListedNumber{Number: number}.SameNumber)
}

func (ErrorsTable) sectionStart(lines []string, heading string) int {
	for index, line := range lines {
		if strings.TrimSpace(line) == heading {
			return index + 1
		}
	}
	return len(lines)
}

func (ErrorsTable) sectionEnd(lines []string, start int) int {
	for index := start; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "## ") {
			return index
		}
	}
	return len(lines)
}

func (table ErrorsTable) owners(lines []string, start int, end int) Owners {
	pattern := regexp.MustCompile("^\\| MS(\\d{4})-MS(\\d{4}) \\| [^|`]*`([^`]+)`")
	owners := Owners{}
	for index := start; index < end; index++ {
		owners = append(owners, table.owner(pattern.FindStringSubmatch(lines[index]))...)
	}
	return owners
}

func (table ErrorsTable) owner(match []string) Owners {
	if match == nil {
		return nil
	}
	return Owners{Owner{}.FromBlock(table.number(match[1]), table.number(match[2]), match[3])}
}

func (ErrorsTable) number(digits string) int {
	number, _ := strconv.Atoi(digits)
	return number
}

func (table ErrorsTable) rows(lines []string, start int, end int) []ListedNumber {
	pattern := regexp.MustCompile(`^\| MS(\d{4}) \| ([^|]*?) \|`)
	rows := []ListedNumber{}
	for index := start; index < end; index++ {
		rows = append(rows, table.row(pattern.FindStringSubmatch(lines[index]), index+1)...)
	}
	return rows
}

func (table ErrorsTable) row(match []string, line int) []ListedNumber {
	if match == nil {
		return nil
	}
	return []ListedNumber{{Number: table.number(match[1]), App: match[2], Line: line}}
}

func (table ErrorsTable) outsideBlockProblems() []Problem {
	problems := []Problem{}
	for _, row := range table.Rows {
		problems = append(problems, table.outsideBlockProblem(row)...)
	}
	return problems
}

func (table ErrorsTable) outsideBlockProblem(row ListedNumber) []Problem {
	owner, _ := table.Owners.Holding(row.Number)
	if owner.Name == row.App {
		return nil
	}
	return []Problem{Problem{}.ListedOutsideBlock(row)}
}

func (table ErrorsTable) twiceProblems() []Problem {
	seen := map[int]bool{}
	problems := []Problem{}
	for _, row := range table.Rows {
		problems = append(problems, table.twiceProblem(seen, row)...)
		seen[row.Number] = true
	}
	return problems
}

func (ErrorsTable) twiceProblem(seen map[int]bool, row ListedNumber) []Problem {
	if !seen[row.Number] {
		return nil
	}
	return []Problem{Problem{}.ListedTwice(row)}
}

func (table ErrorsTable) orderProblems() []Problem {
	problems := []Problem{}
	for index := 1; index < len(table.Rows); index++ {
		problems = append(problems, table.orderProblem(table.Rows[index], table.Rows[index-1])...)
	}
	return problems
}

func (ErrorsTable) orderProblem(row ListedNumber, previous ListedNumber) []Problem {
	if row.Number >= previous.Number {
		return nil
	}
	return []Problem{Problem{}.NotSorted(row, previous)}
}
