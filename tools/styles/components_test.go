package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeCompiler struct {
	outputs map[string]string
}

func (fake fakeCompiler) compile(template []byte) (string, error) {
	output, found := fake.outputs[strings.TrimSpace(string(template))]
	if !found {
		return "", errors.New("unexpected template")
	}
	return output, nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func sampleClient(t *testing.T) string {
	t.Helper()
	client := filepath.Join(t.TempDir(), "apps", "web", "client")
	writeFile(t, filepath.Join(client, "components/app/shell/tac.html"), "shell")
	writeFile(t, filepath.Join(client, "components/app/shell/tac.css"), ".app {}\n")
	writeFile(t, filepath.Join(client, "components/app/root/tac.html"), "root")
	writeFile(t, filepath.Join(client, "components/app/plain/tac.html"), "plain")
	writeFile(t, filepath.Join(client, foundationPath), ":root {}\n")
	return client
}

func sampleCompiler() fakeCompiler {
	return fakeCompiler{outputs: map[string]string{
		"shell": sampleOutput,
		"root":  "@layer utilities {\n  .grid {\n    display: grid;\n  }\n}\n",
		"plain": "/*! tailwindcss v4.3.5 | MIT License | https://tailwindcss.com */\n@layer utilities;\n",
	}}
}

func TestFindComponentsReturnsSortedFoldersWithTemplates(t *testing.T) {
	client := sampleClient(t)
	writeFile(t, filepath.Join(client, "components/app/notes/readme.txt"), "x")
	folders, err := findComponents(filepath.Join(client, "components"))
	want := []string{
		filepath.Join(client, "components/app/plain"),
		filepath.Join(client, "components/app/root"),
		filepath.Join(client, "components/app/shell"),
	}
	if err != nil || !reflect.DeepEqual(folders, want) {
		t.Fatalf("got %v, %v; want %v", folders, err, want)
	}
}

func TestBuildComponentsRejectsHardCodedColors(t *testing.T) {
	client := sampleClient(t)
	fake := fakeCompiler{outputs: map[string]string{"shell": "@layer utilities {\n  .x {\n    color: #000;\n  }\n}\n"}}
	_, err := buildComponents(fake, []string{filepath.Join(client, "components/app/shell")})
	if err == nil || !strings.Contains(err.Error(), "hard-coded colors") {
		t.Fatalf("got %v", err)
	}
}

func generateSample(t *testing.T, client string, check bool) (string, error) {
	t.Helper()
	folders, err := findComponents(filepath.Join(client, "components"))
	if err != nil {
		t.Fatal(err)
	}
	components, err := buildComponents(sampleCompiler(), folders)
	if err != nil {
		t.Fatal(err)
	}
	changes, _, err := planChanges(client, components)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	err = apply(check, client, changes, &log)
	return log.String(), err
}

func TestGenerateWritesComponentRegionsAndFoundation(t *testing.T) {
	client := sampleClient(t)
	if _, err := generateSample(t, client, false); err != nil {
		t.Fatal(err)
	}
	shell := readFile(t, filepath.Join(client, "components/app/shell/tac.css"))
	if !strings.HasPrefix(shell, ".app {}\n\n"+regionStart+"\n") || strings.Contains(shell, "@property") {
		t.Fatalf("unexpected shell styles:\n%s", shell)
	}
	root := readFile(t, filepath.Join(client, "components/app/root/tac.css"))
	if !strings.HasPrefix(root, regionStart+"\n@layer utilities {") {
		t.Fatalf("unexpected root styles:\n%s", root)
	}
	foundation := readFile(t, filepath.Join(client, foundationPath))
	if !strings.HasPrefix(foundation, layerOrder+"\n\n:root {}\n\n"+regionStart+"\n"+foundationHeader+"\n@property --tw-border-style {") {
		t.Fatalf("unexpected foundation:\n%s", foundation)
	}
}

func TestGenerateDoesNotCreateStylesForComponentWithoutUtilities(t *testing.T) {
	client := sampleClient(t)
	if _, err := generateSample(t, client, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(client, "components/app/plain/tac.css")); !os.IsNotExist(err) {
		t.Fatalf("tac.css was created for a component without utilities: %v", err)
	}
}

func TestCheckListsStaleFilesWithoutWriting(t *testing.T) {
	client := sampleClient(t)
	log, err := generateSample(t, client, true)
	if !errors.Is(err, errStale) {
		t.Fatalf("got %v, want errStale", err)
	}
	for _, name := range []string{"apps/web/client/components/app/root/tac.css", "apps/web/client/components/app/shell/tac.css", "apps/web/client/shared/styles/app.css"} {
		if !strings.Contains(log, "styles: stale "+name) {
			t.Fatalf("log does not list %s:\n%s", name, log)
		}
	}
	if readFile(t, filepath.Join(client, "components/app/shell/tac.css")) != ".app {}\n" {
		t.Fatal("check mode wrote a file")
	}
}

func TestCheckPassesAfterGenerate(t *testing.T) {
	client := sampleClient(t)
	if _, err := generateSample(t, client, false); err != nil {
		t.Fatal(err)
	}
	if _, err := generateSample(t, client, true); err != nil {
		t.Fatalf("check failed right after generate: %v", err)
	}
}

func TestPlanChangesRequiresFoundationStylesheet(t *testing.T) {
	client := sampleClient(t)
	if err := os.Remove(filepath.Join(client, foundationPath)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := planChanges(client, nil); err == nil {
		t.Fatal("expected an error when app.css is missing")
	}
}

func TestFindRootWalksUpToRepository(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools/styles/go.mod"), "module x\n")
	writeFile(t, filepath.Join(root, "apps/web/client/components/a/tac.html"), "")
	found, err := findRoot(filepath.Join(root, "tools", "styles"))
	if err != nil || found != root {
		t.Fatalf("got %q, %v; want %q", found, err, root)
	}
}

func TestClientFolderRejectsPathInAppName(t *testing.T) {
	if _, err := clientFolder(options{app: "../web"}); err == nil {
		t.Fatal("expected an error for an app name with a path")
	}
}
