package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeController(t *testing.T, app string, name string, source string) {
	t.Helper()
	folder := filepath.Join(app, "server", "controllers")
	if err := errors.Join(os.MkdirAll(folder, 0o755), os.WriteFile(filepath.Join(folder, name), []byte(source), 0o644)); err != nil {
		t.Fatal(err)
	}
}

const failingController = `type SampleController struct{}

func (controller *SampleController) GetHealth() Health {
	return Health{}
}
`

func TestAppsChecksOnlyControllerFiles(t *testing.T) {
	app := filepath.Join(t.TempDir(), "sample")
	writeController(t, app, "MessagesController.go", passingController)
	writeController(t, app, "SampleErrorHandler.go", failingController)
	writeController(t, app, "SampleController_test.go", failingController)
	problems, err := ControllerCheck{}.Apps([]string{app})
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAppsReportsProblemsFromEveryApp(t *testing.T) {
	root := t.TempDir()
	writeController(t, filepath.Join(root, "one"), "SampleController.go", failingController)
	writeController(t, filepath.Join(root, "two"), "SampleController.go", failingController)
	problems, err := ControllerCheck{}.Apps([]string{filepath.Join(root, "one"), filepath.Join(root, "two")})
	if err != nil || len(problems) != 2 || problems[1].File != filepath.Join(root, "two", "server", "controllers", "SampleController.go") {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAnAppWithoutControllersHasNoProblems(t *testing.T) {
	problems, err := ControllerCheck{}.Apps([]string{t.TempDir()})
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAMissingAppFolderIsAnError(t *testing.T) {
	_, err := ControllerCheck{}.Apps([]string{filepath.Join(t.TempDir(), "missing")})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestAControllerThatDoesNotParseIsAnError(t *testing.T) {
	app := t.TempDir()
	writeController(t, app, "BrokenController.go", "type BrokenController struct {\n")
	if _, err := (ControllerCheck{}).Apps([]string{app}); err == nil {
		t.Fatal("want a parse error")
	}
}

func TestAControllerThatCannotBeReadIsAnError(t *testing.T) {
	app := t.TempDir()
	folder := filepath.Join(app, "server", "controllers", "FolderController.go")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (ControllerCheck{}).Apps([]string{app}); err == nil {
		t.Fatal("want a read error")
	}
}
