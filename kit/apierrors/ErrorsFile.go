package apierrors

import (
	"os"
	"path/filepath"
)

type ErrorsFile struct {
	Path string
}

func (file ErrorsFile) Nearest(folder string) (ErrorsFile, error) {
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return ErrorsFile{}, err
	}
	return file.firstExisting(absolute, file.ancestors(absolute))
}

func (file ErrorsFile) Check(first int, numbers []int) error {
	text, err := os.ReadFile(file.Path)
	if err != nil {
		return err
	}
	return ErrorCatalog{}.Parse(string(text)).Check(first, numbers)
}

func (ErrorsFile) ancestors(folder string) []string {
	folders := []string{folder}
	for parent := filepath.Dir(folder); parent != folders[len(folders)-1]; parent = filepath.Dir(parent) {
		folders = append(folders, parent)
	}
	return folders
}

func (ErrorsFile) firstExisting(start string, folders []string) (ErrorsFile, error) {
	for _, folder := range folders {
		candidate := filepath.Join(folder, "docs", "mesail", "ERRORS.md")
		if _, err := os.Stat(candidate); err == nil {
			return ErrorsFile{Path: candidate}, nil
		}
	}
	return ErrorsFile{}, MissingErrorsFile{From: start}
}
