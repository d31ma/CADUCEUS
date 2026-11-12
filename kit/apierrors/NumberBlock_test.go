package apierrors

import "testing"

func TestHoldsTheFirstNumber(t *testing.T) {
	if !(NumberBlock{First: 500, Last: 999}).Holds(500) {
		t.Fatal("500 is in MS0500-MS0999")
	}
}

func TestHoldsTheLastNumber(t *testing.T) {
	if !(NumberBlock{First: 500, Last: 999}).Holds(999) {
		t.Fatal("999 is in MS0500-MS0999")
	}
}

func TestDoesNotHoldTheNextBlock(t *testing.T) {
	if (NumberBlock{First: 500, Last: 999}).Holds(1000) {
		t.Fatal("1000 is not in MS0500-MS0999")
	}
}

func TestDoesNotHoldTheBlockBefore(t *testing.T) {
	if (NumberBlock{First: 500, Last: 999}).Holds(499) {
		t.Fatal("499 is not in MS0500-MS0999")
	}
}
