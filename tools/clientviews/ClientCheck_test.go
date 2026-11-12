package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const cleanManifest = `{"name": "Sample", "tac": {"styles": []}}`

const cleanView = `<w-btn :aria-current="row.ariaCurrent" on:click="list.open(row.id)">{row.title}</w-btn>`

const choiceView = `<w-btn :aria-current="row.current ? 'page' : false">{row.title}</w-btn>`

func writeClientFile(t *testing.T, app string, name string, source string) {
	t.Helper()
	path := filepath.Join(app, "client", name)
	if err := errors.Join(os.MkdirAll(filepath.Dir(path), 0o755), os.WriteFile(path, []byte(source), 0o644)); err != nil {
		t.Fatal(err)
	}
}

func cleanApp(t *testing.T, app string) string {
	t.Helper()
	writeClientFile(t, app, "manifest.json", cleanManifest)
	writeClientFile(t, app, "pages/tac.html", "<app-root />")
	writeClientFile(t, app, "components/app/root/tac.html", cleanView)
	writeClientFile(t, app, "components/app/root/tac.css", ".x { content: 'a ? b : c'; }")
	return app
}

func TestACleanAppHasNoProblems(t *testing.T) {
	problems, err := ClientCheck{}.Apps([]string{cleanApp(t, t.TempDir())})
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAManifestThatTurnsTheDependencyCheckOffIsReported(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	writeClientFile(t, app, "manifest.json", `{"tac": {"dependencies": {"check": false}}}`)
	problems, err := ClientCheck{}.Apps([]string{app})
	if err != nil || len(problems) != 1 || problems[0].File != filepath.Join(app, "client", "manifest.json") {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestATernaryInANestedComponentViewIsReported(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	writeClientFile(t, app, "components/sidebar/nav/tac.html", choiceView)
	problems, err := ClientCheck{}.Apps([]string{app})
	if err != nil || len(problems) != 1 || problems[0].File != filepath.Join(app, "client", "components", "sidebar", "nav", "tac.html") {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestATernaryInAPageViewIsReported(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	writeClientFile(t, app, "pages/tac.html", choiceView)
	problems, err := ClientCheck{}.Apps([]string{app})
	if err != nil || len(problems) != 1 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestOnlyFilesNamedTacHtmlAreViews(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	writeClientFile(t, app, "shared/notes.html", choiceView)
	writeClientFile(t, app, "tac.html.orig/readme.txt", choiceView)
	problems, err := ClientCheck{}.Apps([]string{app})
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestProblemsFromEveryAppAreReported(t *testing.T) {
	root := t.TempDir()
	one := cleanApp(t, filepath.Join(root, "one"))
	two := cleanApp(t, filepath.Join(root, "two"))
	writeClientFile(t, one, "components/a/tac.html", choiceView)
	writeClientFile(t, two, "components/b/tac.html", choiceView)
	problems, err := ClientCheck{}.Apps([]string{one, two})
	if err != nil || len(problems) != 2 || problems[1].File != filepath.Join(two, "client", "components", "b", "tac.html") {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAnAppWithoutAManifestIsAnError(t *testing.T) {
	_, err := ClientCheck{}.Apps([]string{t.TempDir()})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestAManifestThatIsNotJsonStopsTheCheck(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	writeClientFile(t, app, "manifest.json", "{")
	if _, err := (ClientCheck{}).Apps([]string{app}); err == nil {
		t.Fatal("want an error")
	}
}

func TestAViewThatCannotBeReadIsAnError(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	view := filepath.Join(app, "client", "components", "locked", "tac.html")
	writeClientFile(t, app, "components/locked/tac.html", cleanView)
	if err := os.Chmod(view, 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := (ClientCheck{}).Apps([]string{app}); err == nil {
		t.Fatal("want a read error")
	}
}

func TestAComponentsFolderThatCannotBeListedIsAnError(t *testing.T) {
	app := cleanApp(t, t.TempDir())
	folder := filepath.Join(app, "client", "components", "app")
	if err := os.Chmod(folder, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(folder, 0o755) })
	if _, err := (ClientCheck{}).Apps([]string{app}); err == nil {
		t.Fatal("want a walk error")
	}
}
