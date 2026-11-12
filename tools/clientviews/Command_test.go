package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAnswersZeroForACleanApp(t *testing.T) {
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{cleanApp(t, t.TempDir())})
	if code != 0 || output.String() != "clientviews: the dependency check is on and every view shows only service values\n" {
		t.Fatalf("got %d, %q", code, output.String())
	}
}

func TestRunAnswersOneAndListsEachProblem(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	writeClientFile(t, app, "components/sidebar/nav/tac.html", choiceView)
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{app})
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if code != 1 || len(lines) != 2 || !strings.Contains(lines[0], "nav/tac.html:1: the view holds a ? : expression") || lines[1] != "clientviews: 1 problems break SLICES rule 19 (client layers)" {
		t.Fatalf("got %d, %q", code, output.String())
	}
}

func TestRunAnswersTwoWhenAnAppCannotBeRead(t *testing.T) {
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{filepath.Join(t.TempDir(), "missing")})
	if code != 2 || !strings.HasPrefix(output.String(), "clientviews: ") {
		t.Fatalf("got %d, %q", code, output.String())
	}
}
