package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=gateplan", "-c", "user.email=gateplan@example.com", "-c", "commit.gpgsign=false"}, arguments...)...)
	output, failure := command.CombinedOutput()
	if failure != nil {
		t.Fatalf("git %v: %v\n%s", arguments, failure, output)
	}
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if failure := os.MkdirAll(filepath.Dir(path), 0o755); failure != nil {
		t.Fatal(failure)
	}
	if failure := os.WriteFile(path, []byte(content), 0o644); failure != nil {
		t.Fatal(failure)
	}
}

func newRepository(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "develop")
	writeFile(t, root, "apps/channels/server/packages/kit.list", "events\noutbox\n")
	writeFile(t, root, "apps/web/client/index.html", "<p></p>\n")
	writeFile(t, root, "kit/events/Catalog.go", "package events\n")
	writeFile(t, root, ".gitignore", ".env\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "start")
	git(t, root, "branch", "base")
	return root
}

func run(root string, arguments ...string) (int, string, string) {
	output := bytes.Buffer{}
	errors := bytes.Buffer{}
	code := Command{Output: &output, Errors: &errors}.Run(append([]string{"--root", root}, arguments...))
	return code, output.String(), errors.String()
}

func TestRunChoosesTheAppOfACommittedChange(t *testing.T) {
	root := newRepository(t)
	writeFile(t, root, "apps/web/client/index.html", "<p>new</p>\n")
	git(t, root, "commit", "-q", "-am", "web")
	code, output, _ := run(root, "--base", "base")
	if code != 0 || output != "web\town files: apps/web/client/index.html\n" {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunAddsUncommittedAndNewFilesButNotIgnoredOnes(t *testing.T) {
	root := newRepository(t)
	writeFile(t, root, "kit/events/Catalog.go", "package events\n\ntype Catalog struct{}\n")
	writeFile(t, root, "apps/web/client/new.html", "<p></p>\n")
	writeFile(t, root, "apps/channels/.env", "YON=1\n")
	code, output, _ := run(root, "--base", "base")
	if code != 0 || output != "channels\tuses kit/events: kit/events/Catalog.go\nweb\town files: apps/web/client/new.html\n" {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunChoosesEveryAppWhenNothingDiffers(t *testing.T) {
	root := newRepository(t)
	code, output, _ := run(root, "--base", "base")
	if code != 0 || output != "*\tno file differs from base\n" {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunChoosesEveryAppWhenTheBaseIsMissing(t *testing.T) {
	root := newRepository(t)
	code, output, _ := run(root, "--base", "origin/develop")
	if code != 0 || !strings.HasPrefix(output, "*\tthe changed files against origin/develop could not be listed: git diff") {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunFailsWithoutAnAppsFolder(t *testing.T) {
	code, output, errors := run(t.TempDir())
	if code != 2 || output != "" || !strings.HasPrefix(errors, "gateplan: open ") {
		t.Fatalf("got %d, %q, %q", code, output, errors)
	}
}

func TestRunFailsOnAnUnknownOption(t *testing.T) {
	code, output, errors := run(t.TempDir(), "--everything")
	if code != 2 || output != "" || !strings.Contains(errors, "flag provided but not defined: -everything") {
		t.Fatalf("got %d, %q, %q", code, output, errors)
	}
}
