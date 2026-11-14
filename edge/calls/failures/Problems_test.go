package failures

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"mesail.dev/packages/apierrors"
)

func TestEveryNumberIsListedInTheCallsBlock(t *testing.T) {
	file, err := apierrors.ErrorsFile{}.Nearest(".")
	if err != nil {
		t.Fatal(err)
	}
	numbers := []int{}
	for _, problem := range (Problems{}).All() {
		numbers = append(numbers, problem.Number())
	}
	err = file.Check(6500, numbers)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNumbersAreUnique(t *testing.T) {
	seen := map[int]bool{}
	for _, problem := range (Problems{}).All() {
		if seen[problem.Number()] {
			t.Fatalf("number %d is used twice", problem.Number())
		}
		seen[problem.Number()] = true
	}
}

func TestMessageHasTheNumber(t *testing.T) {
	problem := Problems{}.CallFull()
	if problem.Error() != "[MS6506]: The call is full." {
		t.Fatalf("message %q", problem.Error())
	}
}

func TestFromFindsAWrappedProblem(t *testing.T) {
	wrapped := fmt.Errorf("joining: %w", Problems{}.InvalidOffer())
	if (Problems{}).From(wrapped).Number() != 6509 {
		t.Fatal("the wrapped problem was not found")
	}
}

func TestFromTurnsAnOtherErrorIntoUnavailable(t *testing.T) {
	problem := Problems{}.From(errors.New("disk full"))
	if problem.Status != http.StatusInternalServerError || problem.Number() != 6500 {
		t.Fatalf("problem %+v", problem)
	}
}
