package oidc

import (
	"errors"
	"testing"
)

func TestAuthorizationNeedsItsSettings(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "{}")
	_, err := ServiceTokensFactory{}.Configured().Authorization("example.com")
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestJsonFileCacheReportsAValueItCannotWrite(t *testing.T) {
	if err := (JsonFileCache{Folder: t.TempDir()}).Store("k", make(chan int)); err == nil {
		t.Fatal("expected a failure")
	}
}
