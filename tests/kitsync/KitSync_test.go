package kitsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const moduleFolder = "edge/fixture"

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}

func writeFile(t *testing.T, path string, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make folder for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func copyScript(t *testing.T, fixture string, name string) {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(repositoryRoot(t), "scripts", name))
	if err != nil {
		t.Fatalf("read scripts/%s: %v", name, err)
	}
	writeFile(t, filepath.Join(fixture, "scripts", name), string(text))
}

func kitFile(fixture string) string {
	return filepath.Join(fixture, "kit", "oidc", "Issuer.go")
}

func vendoredKitFile(fixture string) string {
	return filepath.Join(fixture, moduleFolder, "vendor", "mesail.dev", "packages", "oidc", "Issuer.go")
}

func writeKit(t *testing.T, fixture string) {
	t.Helper()
	writeFile(t, filepath.Join(fixture, "kit", "go.mod"), "module mesail.dev/packages\n\ngo 1.27\n")
	writeFile(t, kitFile(fixture), "package oidc\n\nconst Issuer = \"first\"\n")
}

func writeVendoringModule(t *testing.T, fixture string) {
	t.Helper()
	module := filepath.Join(fixture, moduleFolder)
	writeFile(t, filepath.Join(module, "go.mod"), "module mesail.dev/edge/fixture\n\ngo 1.27\n\nrequire mesail.dev/packages v0.0.0\n\nreplace mesail.dev/packages => ../../kit\n")
	writeFile(t, filepath.Join(module, "main.go"), "package main\n\nimport \"mesail.dev/packages/oidc\"\n\nfunc main() {\n\tprintln(oidc.Issuer)\n}\n")
	command := exec.Command("go", "mod", "vendor")
	command.Dir = module
	command.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go mod vendor in the fixture: %v\n%s", err, output)
	}
}

func newFixture(t *testing.T) string {
	t.Helper()
	fixture := t.TempDir()
	copyScript(t, fixture, "kit-sync.sh")
	copyScript(t, fixture, "go-cache.sh")
	writeKit(t, fixture)
	writeVendoringModule(t, fixture)
	return fixture
}

func changeKit(t *testing.T, fixture string) {
	t.Helper()
	writeFile(t, kitFile(fixture), "package oidc\n\nconst Issuer = \"second\"\n")
}

func runKitSync(t *testing.T, fixture string, arguments ...string) (string, error) {
	t.Helper()
	command := exec.Command(filepath.Join(fixture, "scripts", "kit-sync.sh"), arguments...)
	command.Dir = fixture
	command.Env = append(os.Environ(), "MESAIL_GOCACHE="+t.TempDir())
	output, err := command.CombinedOutput()
	return string(output), err
}

func syncKit(t *testing.T, fixture string) {
	t.Helper()
	if output, err := runKitSync(t, fixture); err != nil {
		t.Fatalf("kit-sync: %v\n%s", err, output)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(text)
}

func TestCheckPassesWhenVendoredCopyIsCurrent(t *testing.T) {
	fixture := newFixture(t)

	output, err := runKitSync(t, fixture, "--check")

	if err != nil || !strings.Contains(output, "kit-sync: check ok") {
		t.Fatalf("kit-sync --check on a current copy: err %v\n%s", err, output)
	}
}

func TestCheckNamesModuleWhenVendoredCopyIsStale(t *testing.T) {
	fixture := newFixture(t)
	changeKit(t, fixture)

	output, err := runKitSync(t, fixture, "--check")

	if err == nil || !strings.Contains(output, "kit-sync: "+moduleFolder+"/vendor is not what go mod vendor makes from kit/") {
		t.Fatalf("kit-sync --check on a stale copy must fail and name %s: err %v\n%s", moduleFolder, err, output)
	}
}

func TestSyncVendorsTheKitAgain(t *testing.T) {
	fixture := newFixture(t)
	changeKit(t, fixture)

	output, err := runKitSync(t, fixture)

	if err != nil || !strings.Contains(output, "kit-sync: go mod vendor -> "+moduleFolder+"/vendor") {
		t.Fatalf("kit-sync must vendor %s again: err %v\n%s", moduleFolder, err, output)
	}
	if readFile(t, vendoredKitFile(fixture)) != readFile(t, kitFile(fixture)) {
		t.Fatalf("the vendored copy does not match kit/oidc/Issuer.go after kit-sync")
	}
}

func TestCheckPassesAfterSync(t *testing.T) {
	fixture := newFixture(t)
	changeKit(t, fixture)
	syncKit(t, fixture)

	output, err := runKitSync(t, fixture, "--check")

	if err != nil || !strings.Contains(output, "kit-sync: check ok") {
		t.Fatalf("kit-sync --check after kit-sync: err %v\n%s", err, output)
	}
}
