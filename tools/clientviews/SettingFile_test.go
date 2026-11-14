package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAMissingSettingFileIsNotFoundAndNotAnError(t *testing.T) {
	source, found, err := SettingFile{Name: filepath.Join(t.TempDir(), ".env")}.Read()
	if source != nil || found || err != nil {
		t.Fatalf("got %q, %v, %v", source, found, err)
	}
}

func TestASettingFileIsRead(t *testing.T) {
	name := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(name, []byte("A=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, found, err := SettingFile{Name: name}.Read()
	if string(source) != "A=1" || !found || err != nil {
		t.Fatalf("got %q, %v, %v", source, found, err)
	}
}

func TestASettingFileThatCannotBeReadIsAnError(t *testing.T) {
	name := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(name, []byte("A=1"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, found, err := SettingFile{Name: name}.Read()
	if found || err == nil {
		t.Fatalf("got %v, %v", found, err)
	}
}
