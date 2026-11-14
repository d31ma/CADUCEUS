package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestNumbersReadsALiteralResultOfANumberMethod(t *testing.T) {
	source := `type NotFound struct{}

func (failure *NotFound) Number() int {
	return 3508
}
`
	expectNumbers(t, source, []int{3508})
}

func TestNumbersReadsAKeyedFieldThatNumberReturns(t *testing.T) {
	source := `type Invalid struct {
	Field       string
	ErrorNumber int
}

func (Invalid) Name() *Invalid {
	return &Invalid{Field: "name", ErrorNumber: 2003}
}

func (failure *Invalid) Number() int {
	return failure.ErrorNumber
}

type Domain struct {
	ErrorNumber int
}

func (Domain) Size() Domain {
	return Domain{ErrorNumber: 3503}
}
`
	expectNumbers(t, source, []int{2003, 3503})
}

func TestNumbersReadsAPositionalFieldThatNumberReturns(t *testing.T) {
	source := `type ErrorCase struct {
	number int
	text   string
}

func (failure ErrorCase) Number() int {
	return failure.number
}

type Conflict struct {
	ErrorCase
}

func (Conflict) LabelExists() *Conflict {
	return &Conflict{ErrorCase{510, "This passkey name is already used."}}
}
`
	expectNumbers(t, source, []int{510})
}

func TestNumbersReadsANumberPassedToAHelper(t *testing.T) {
	source := `type Invalid struct {
	Reason      string
	ErrorNumber int
}

func (request Invalid) Producer() Invalid {
	return request.field("producer", 3005)
}

func (Invalid) field(name string, number int) Invalid {
	return Invalid{Reason: name, ErrorNumber: number}
}

func (failure Invalid) Number() int {
	return failure.ErrorNumber
}
`
	expectNumbers(t, source, []int{3005})
}

func TestNumbersIgnoresOtherIntegers(t *testing.T) {
	source := `type Page struct {
	Size int
}

func (Page) First() Page {
	return Page{Size: 50}
}
`
	expectNumbers(t, source, []int{})
}

func expectNumbers(t *testing.T, source string, expected []int) {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "Sample.go", "package sample\n\n"+source, 0)
	if err != nil {
		t.Fatal(err)
	}
	numbers := []int{}
	for _, used := range (NumberReader{Positions: positions, Files: []*ast.File{file}}).Numbers() {
		numbers = append(numbers, used.Number)
	}
	if !reflect.DeepEqual(numbers, expected) {
		t.Fatalf("got %v, want %v", numbers, expected)
	}
}
