package apierrors

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNearestFindsTheRepositoryErrorsFile(t *testing.T) {
	file, err := ErrorsFile{}.Nearest(".")
	if err != nil || !strings.HasSuffix(file.Path, filepath.Join("docs", "mesail", "ERRORS.md")) {
		t.Fatalf("got %q, %v", file.Path, err)
	}
}

func TestTheRepositoryErrorsFileListsEveryUnexpectedErrorOnce(t *testing.T) {
	file, err := ErrorsFile{}.Nearest(".")
	if err != nil {
		t.Fatal(err)
	}
	err = errors.Join(
		file.Check(0, []int{0}), file.Check(500, []int{500}), file.Check(1000, []int{1000}),
		file.Check(1500, []int{1500}), file.Check(2000, []int{2000}), file.Check(2500, []int{2500}),
		file.Check(3000, []int{3000}), file.Check(3500, []int{3500}), file.Check(4000, []int{4000}),
		file.Check(4500, []int{4500}), file.Check(5000, []int{5000}), file.Check(5500, []int{5500}),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNearestReportsAFolderWithoutTheFile(t *testing.T) {
	var missing MissingErrorsFile
	_, err := ErrorsFile{}.Nearest(t.TempDir())
	if !errors.As(err, &missing) || !strings.Contains(err.Error(), "no docs/mesail/ERRORS.md in ") {
		t.Fatalf("got %v", err)
	}
}

func TestNearestFindsTheFileFromADeepFolder(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "apps", "identity", "server", "controllers")
	errorsPath := filepath.Join(root, "docs", "mesail", "ERRORS.md")
	if err := errors.Join(os.MkdirAll(deep, 0o755), os.MkdirAll(filepath.Dir(errorsPath), 0o755), os.WriteFile(errorsPath, []byte("| MS0500-MS0999 | identity | MS0500 |\n| MS0500 | identity |\n"), 0o644)); err != nil {
		t.Fatal(err)
	}
	file, err := ErrorsFile{}.Nearest(deep)
	if err != nil || file.Check(500, []int{500}) != nil {
		t.Fatalf("got %q, %v", file.Path, err)
	}
}

func TestCheckReportsAFileThatCannotBeRead(t *testing.T) {
	file := ErrorsFile{Path: filepath.Join(t.TempDir(), "missing.md")}
	if err := file.Check(500, []int{500}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}
