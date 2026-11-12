package apierrors

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type ErrorCatalog struct {
	blocks []NumberBlock
	listed map[int]int
}

func (ErrorCatalog) Parse(text string) ErrorCatalog {
	empty := ErrorCatalog{listed: map[int]int{}}
	return empty.withLines(strings.Split(text, "\n"), regexp.MustCompile(`^\| MS(\d{4})(?:-MS(\d{4}))? \|`))
}

func (catalog ErrorCatalog) Check(first int, numbers []int) error {
	block, err := catalog.block(first)
	if err != nil {
		return err
	}
	return errors.Join(catalog.problems(block, numbers)...)
}

func (catalog ErrorCatalog) withLines(lines []string, row *regexp.Regexp) ErrorCatalog {
	for _, line := range lines {
		catalog = catalog.withRow(row.FindStringSubmatch(line))
	}
	return catalog
}

func (catalog ErrorCatalog) withRow(match []string) ErrorCatalog {
	switch {
	case match == nil:
		return catalog
	case match[2] != "":
		catalog.blocks = append(catalog.blocks, NumberBlock{First: catalog.number(match[1]), Last: catalog.number(match[2])})
		return catalog
	}
	catalog.listed[catalog.number(match[1])]++
	return catalog
}

func (ErrorCatalog) number(digits string) int {
	number, _ := strconv.Atoi(digits)
	return number
}

func (catalog ErrorCatalog) block(first int) (NumberBlock, error) {
	for _, block := range catalog.blocks {
		if block.First == first {
			return block, nil
		}
	}
	return NumberBlock{}, UnknownBlock{First: first}
}

func (catalog ErrorCatalog) problems(block NumberBlock, numbers []int) []error {
	problems := []error{}
	for _, number := range numbers {
		problems = append(problems, catalog.problem(block, number))
	}
	return problems
}

func (catalog ErrorCatalog) problem(block NumberBlock, number int) error {
	switch {
	case !block.Holds(number):
		return NumberOutsideBlock{Number: number, Block: block}
	case catalog.listed[number] != 1:
		return NumberNotListedOnce{Number: number, Times: catalog.listed[number]}
	}
	return nil
}
