package notifications

import (
	"errors"
	"reflect"
	"testing"
)

func expectSame(t *testing.T, got any, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func expectNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func expectInvalidSettings(t *testing.T, err error, field string) {
	t.Helper()
	var invalid InvalidSettings
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidSettings, got %v", err)
	}
	expectSame(t, invalid.Field, field)
}

func expectInvalidPage(t *testing.T, err error) {
	t.Helper()
	var invalid InvalidPage
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidPage, got %v", err)
	}
}
