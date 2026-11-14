package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const everyAppButChannels = "attachments\ndirectory\nidentity\nmail-in\nmail-out\nmessaging\nnotify\nrealtime\nsearch\n"

const errorsHead = "# Mesail error numbers\n\n## Blocks\n\n| Numbers | Owner | First number |\n| --- | --- | --- |\n| MS0000-MS0499 | `kit` | MS0000 |\n| MS2000-MS2499 | `channels` | MS2000 |\n\n## Numbers\n\n| Number | App | Status |\n| --- | --- | --- |\n| MS0000 | kit | 500 |\n| MS2000 | channels | 500 |\n"

const conflictType = `type Conflict struct {
	Message     string
	ErrorNumber int
}

func (Conflict) NameUsed() *Conflict {
	return &Conflict{Message: "the name is used", ErrorNumber: 2001}
}

func (failure *Conflict) Number() int {
	return failure.ErrorNumber
}
`

func TestRunAnswersZeroWhenCodeAndListAgree(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2001 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	code, output := run(t, root)
	if code != 0 || output != "errornumbers: every error number in code is listed once in docs/mesail/ERRORS.md, inside its app's block\n" {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunFindsANumberInCodeThatIsNotListed(t *testing.T) {
	root := newRepository(t, errorsHead)
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	expectProblem(t, root, "apps/channels/server/shared/Conflict.go:7: MS2001 is used in code but is not in the Numbers table")
}

func TestRunFindsAListedNumberNoCodeUses(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2001 | channels | 409 |\n| MS2002 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	expectProblem(t, root, "docs/mesail/ERRORS.md:17: MS2002 (channels) is listed but no code of channels uses it")
}

func TestRunFindsANumberInCodeOutsideTheAppsBlock(t *testing.T) {
	root := newRepository(t, errorsHead)
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", strings.Replace(conflictType, "2001", "2600", 1))
	expectProblem(t, root, "MS2600 is outside the block of channels (MS2000-MS2499)")
}

func TestRunFindsAListedNumberOutsideTheAppsBlock(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2001 | channels | 409 |\n| MS2600 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	expectProblem(t, root, "docs/mesail/ERRORS.md:17: MS2600 is listed for channels but is outside the block of channels")
}

func TestRunFindsANumberListedTwice(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2001 | channels | 409 |\n| MS2001 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	expectProblem(t, root, "docs/mesail/ERRORS.md:17: MS2001 is listed more than once")
}

func TestRunFindsANumberUsedTwice(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2001 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType+`
func (Conflict) Archived() *Conflict {
	return &Conflict{Message: "archived", ErrorNumber: 2001}
}
`)
	expectProblem(t, root, "apps/channels/server/shared/Conflict.go:15: MS2001 is used twice; it is already used at apps/channels/server/shared/Conflict.go:7")
}

func TestRunFindsRowsThatAreNotSorted(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2002 | channels | 409 |\n| MS2001 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType+`
func (Conflict) Archived() *Conflict {
	return &Conflict{Message: "archived", ErrorNumber: 2002}
}
`)
	expectProblem(t, root, "docs/mesail/ERRORS.md:17: MS2001 comes after MS2002; sort the rows by number")
}

func TestRunReadsOnlyTheNumbersTable(t *testing.T) {
	misplaced := strings.Replace(errorsHead, "| MS2000-MS2499 | `channels` | MS2000 |\n", "| MS2000-MS2499 | `channels` | MS2000 |\n| MS2001 | channels | 409 |\n", 1)
	root := newRepository(t, misplaced)
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	expectProblem(t, root, "MS2001 is used in code but is not in the Numbers table")
}

func TestRunSkipsAppsThatAreNotConverted(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2002 | channels | 409 |\n")
	writeFile(t, root, "tools/controllers/unconverted.list", everyAppButChannels+"channels\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	code, output := run(t, root)
	if code != 0 {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunLeavesKitNumbersInAppCopiesToTheKit(t *testing.T) {
	withKitRow := strings.Replace(errorsHead, "| MS0000 | kit | 500 |\n", "| MS0000 | kit | 500 |\n| MS0001 | kit | 400 |\n", 1)
	root := newRepository(t, withKitRow+"| MS2001 | channels | 409 |\n")
	kitError := strings.Replace(strings.Replace(conflictType, "2001", "1", 1), "type Conflict", "type Invalid", 1)
	kitError = strings.NewReplacer("Conflict{", "Invalid{", "*Conflict", "*Invalid", "(Conflict)", "(Invalid)").Replace(kitError)
	writeFile(t, root, "kit/events/Invalid.go", "package events\n\n"+kitError)
	writeFile(t, root, "apps/channels/server/packages/events/Invalid.go", "package events\n\n"+kitError)
	writeFile(t, root, "apps/channels/server/packages/kit.list", "events\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	code, output := run(t, root)
	if code != 0 {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunSkipsTestFiles(t *testing.T) {
	root := newRepository(t, errorsHead+"| MS2001 | channels | 409 |\n")
	writeFile(t, root, "apps/channels/server/shared/Conflict.go", conflictType)
	writeFile(t, root, "apps/channels/server/tests/ConflictTest.go", "func Built() *Conflict {\n\treturn &Conflict{ErrorNumber: 2001}\n}\n")
	code, output := run(t, root)
	if code != 0 {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunReadsANewBlockFromTheBlocksTable(t *testing.T) {
	withBlock := strings.Replace(errorsHead, "\n\n## Numbers", "\n| MS6000-MS6499 | `devicebox` (ADR 0011) | MS6000 |\n\n## Numbers", 1)
	root := newRepository(t, withBlock+"| MS6000 | devicebox | 500 |\n| MS6001 | devicebox | 409 |\n")
	writeFile(t, root, "apps/devicebox/server/shared/Conflict.go", strings.Replace(conflictType, "2001", "6001", 1))
	code, output := run(t, root)
	if code != 0 {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunReadsTheFolderABlockNames(t *testing.T) {
	withBlock := strings.Replace(errorsHead, "\n\n## Numbers", "\n| MS5500-MS5999 | the router (`deploy/router`) | MS5500 |\n\n## Numbers", 1)
	root := newRepository(t, withBlock+"| MS5500 | router | 502 |\n")
	writeFile(t, root, "deploy/router/Conflict.go", "package main\n\n"+strings.Replace(conflictType, "2001", "5501", 1))
	expectProblem(t, root, "deploy/router/Conflict.go:9: MS5501 is used in code but is not in the Numbers table")
}

func TestRunAnswersTwoWithoutTheRepositoryArgument(t *testing.T) {
	var output bytes.Buffer
	code := Command{Output: &output}.Run(nil)
	if code != 2 || output.String() != "errornumbers: give one argument: the repository folder\n" {
		t.Fatalf("got %d, %q", code, output.String())
	}
}

func TestRunAnswersTwoWhenErrorsFileIsMissing(t *testing.T) {
	code, output := run(t, t.TempDir())
	if code != 2 || !strings.HasPrefix(output, "errornumbers: open ") {
		t.Fatalf("got %d, %q", code, output)
	}
}

func TestRunAnswersTwoWhenCodeCannotBeParsed(t *testing.T) {
	root := newRepository(t, errorsHead)
	writeFile(t, root, "apps/channels/server/shared/Broken.go", "func {\n")
	code, output := run(t, root)
	if code != 2 || !strings.Contains(output, "Broken.go") {
		t.Fatalf("got %d, %q", code, output)
	}
}

func newRepository(t *testing.T, errorsFile string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "docs/mesail/ERRORS.md", errorsFile)
	writeFile(t, root, "tools/controllers/unconverted.list", everyAppButChannels)
	return root
}

func writeFile(t *testing.T, root string, name string, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, root string) (int, string) {
	t.Helper()
	var output bytes.Buffer
	code := Command{Output: &output}.Run([]string{root})
	return code, output.String()
}

func expectProblem(t *testing.T, root string, problem string) {
	t.Helper()
	code, output := run(t, root)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if code != 1 || len(lines) != 2 || !strings.Contains(lines[0], problem) || !strings.HasPrefix(lines[1], "errornumbers: 1 problems;") {
		t.Fatalf("got %d, %q", code, output)
	}
}
