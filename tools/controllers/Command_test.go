package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAnswersZeroWhenEveryMethodPasses(t *testing.T) {
	app := t.TempDir()
	writeController(t, app, "MessagesController.go", passingController)
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{app})
	if code != 0 || output.String() != "controllers: every route method returns one Service call\n" {
		t.Fatalf("got %d, %q", code, output.String())
	}
}

func TestRunAnswersOneAndListsEachProblem(t *testing.T) {
	app := t.TempDir()
	writeController(t, app, "SampleController.go", failingController)
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{app})
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if code != 1 || len(lines) != 2 || !strings.Contains(lines[0], "SampleController.go:3: SampleController.GetHealth") || !strings.HasPrefix(lines[1], "controllers: 1 route methods break ADR 0009") {
		t.Fatalf("got %d, %q", code, output.String())
	}
}

func TestRunAnswersTwoWhenAnAppCannotBeRead(t *testing.T) {
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{filepath.Join(t.TempDir(), "missing")})
	if code != 2 || !strings.HasPrefix(output.String(), "controllers: ") {
		t.Fatalf("got %d, %q", code, output.String())
	}
}
