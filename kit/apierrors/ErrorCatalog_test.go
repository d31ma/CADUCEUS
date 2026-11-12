package apierrors

import (
	"errors"
	"testing"
)

func sampleCatalog() ErrorCatalog {
	return ErrorCatalog{}.Parse(`# Mesail error numbers

| Numbers | Owner | First number ("unexpected error") |
| --- | --- | --- |
| MS0000-MS0499 | shared kit code | MS0000 |
| MS0500-MS0999 | identity | MS0500 |

| Number | App | Status | code | Message | Case |
| --- | --- | --- | --- | --- | --- |
| MS0000 | kit | 500 | unavailable | Something went wrong. | an unexpected error |
| MS0500 | identity | 500 | unavailable | Something went wrong. | an unexpected error |
| MS0510 | identity | 409 | duplicate | This passkey name is already used. | a name in use |
| MS0510 | identity | 409 | duplicate | This passkey name is already used. | listed twice |
Text that mentions MS0520 is not a row.
`)
}

func TestCheckAcceptsNumbersInTheBlockListedOnce(t *testing.T) {
	if err := sampleCatalog().Check(0, []int{0}); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestCheckAcceptsTheUnexpectedErrorOfABlock(t *testing.T) {
	if err := sampleCatalog().Check(500, []int{500}); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRefusesABlockThatDoesNotExist(t *testing.T) {
	var unknown UnknownBlock
	if err := sampleCatalog().Check(1000, []int{1000}); !errors.As(err, &unknown) || unknown.First != 1000 {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRefusesANumberOutsideTheBlock(t *testing.T) {
	var outside NumberOutsideBlock
	if err := sampleCatalog().Check(0, []int{500}); !errors.As(err, &outside) || outside.Number != 500 {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRefusesANumberListedTwice(t *testing.T) {
	var notListed NumberNotListedOnce
	if err := sampleCatalog().Check(500, []int{510}); !errors.As(err, &notListed) || notListed.Times != 2 {
		t.Fatalf("got %v", err)
	}
}

func TestCheckRefusesANumberThatIsNotListed(t *testing.T) {
	var notListed NumberNotListedOnce
	if err := sampleCatalog().Check(500, []int{520}); !errors.As(err, &notListed) || notListed.Times != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestCheckReportsEveryBadNumber(t *testing.T) {
	err := sampleCatalog().Check(500, []int{500, 510, 520, 0})
	want := "MS0510 is listed 2 times in ERRORS.md; it must be listed once\n" +
		"MS0520 is listed 0 times in ERRORS.md; it must be listed once\n" +
		"MS0000 is outside the block MS0500-MS0999"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
}

func TestUnknownBlockNamesTheBlock(t *testing.T) {
	if got := (UnknownBlock{First: 6000}).Error(); got != "ERRORS.md has no block that starts at MS6000" {
		t.Fatalf("got %q", got)
	}
}
