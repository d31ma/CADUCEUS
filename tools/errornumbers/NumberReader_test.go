package main

import (
	"go/ast"
	"go/importer"
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

func (Invalid) Domain(domain Domain) *Invalid {
	return &Invalid{Field: "domain", ErrorNumber: domain.ErrorNumber}
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

func TestNumbersIgnoresAFieldNamedLikeANumberFieldOnAnotherType(t *testing.T) {
	source := `type ErrorCase struct {
	number int
	text   string
}

func (failure ErrorCase) Number() int {
	return failure.number
}

type Record struct {
	number int
	value  string
}

func (ErrorCase) Missing() ErrorCase {
	return ErrorCase{number: 1003, text: "missing"}
}

func (Record) Lookup() Record {
	return Record{number: 28, value: "mail.example.com"}
}

func (Record) Positional() Record {
	return Record{15, "mail.example.com"}
}
`
	expectNumbers(t, source, []int{1003})
}

func TestNumbersIgnoresABuilderNameOnAnotherType(t *testing.T) {
	source := `type Invalid struct {
	ErrorNumber int
}

func (Invalid) field(number int) Invalid {
	return Invalid{ErrorNumber: number}
}

func (failure Invalid) Number() int {
	return failure.ErrorNumber
}

type Form struct {
	Size int
}

func (Form) field(size int) Form {
	return Form{Size: size}
}

func (Invalid) Name() Invalid {
	return Invalid{}.field(2003)
}

func (Form) Wide() Form {
	return Form{}.field(80)
}
`
	expectNumbers(t, source, []int{2003})
}

func TestNumbersReadsAPositionalFieldOfALiteralWithoutItsTypeName(t *testing.T) {
	source := `type ErrorCase struct {
	number int
	text   string
}

func (failure ErrorCase) Number() int {
	return failure.number
}

func (ErrorCase) cases() []ErrorCase {
	return []ErrorCase{{3001, "first"}, {3002, "second"}}
}
`
	expectNumbers(t, source, []int{3001, 3002})
}

func TestNumbersReadsAFieldThatNumberReadsInsideACall(t *testing.T) {
	source := `type NotFound struct {
	ErrorNumber int
}

func (failure *NotFound) Number() int {
	return max(failure.ErrorNumber, 3507)
}

func (NotFound) Attachment() *NotFound {
	return &NotFound{ErrorNumber: 3507}
}
`
	expectNumbers(t, source, []int{3507})
}

func TestNumbersIgnoresANumberFieldOfATypeThatNoErrorCopies(t *testing.T) {
	source := `type Invalid struct {
	ErrorNumber int
}

func (failure Invalid) Number() int {
	return failure.ErrorNumber
}

type Draft struct {
	ErrorNumber int
}

func (Draft) Size() Draft {
	return Draft{ErrorNumber: 3503}
}
`
	expectNumbers(t, source, []int{})
}

func TestNumbersIgnoresANumberMethodThatDoesNotAnswerAnInteger(t *testing.T) {
	source := `type Version struct {
	number string
}

func (version Version) Number() string {
	return version.number
}

func (Version) Current() Version {
	return Version{number: "26"}
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
	loader := PackageLoader{Positions: positions, Sources: map[string][]*ast.File{"sample": {file}}, Standard: importer.ForCompiler(positions, "source", nil), Checked: map[string]*CheckedPackage{}}
	numbers := []int{}
	for _, used := range (NumberReader{Positions: positions, Packages: loader.All()}).Numbers() {
		numbers = append(numbers, used.Number)
	}
	if !reflect.DeepEqual(numbers, expected) {
		t.Fatalf("got %v, want %v", numbers, expected)
	}
}
