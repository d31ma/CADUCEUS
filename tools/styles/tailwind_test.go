package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetNameForSupportedPlatforms(t *testing.T) {
	cases := map[[2]string]string{
		{"darwin", "arm64"}: "tailwindcss-macos-arm64",
		{"darwin", "amd64"}: "tailwindcss-macos-x64",
		{"linux", "arm64"}:  "tailwindcss-linux-arm64",
		{"linux", "amd64"}:  "tailwindcss-linux-x64",
	}
	for platform, want := range cases {
		got, err := assetName(platform[0], platform[1])
		if err != nil || got != want || len(assetChecksum(got)) != 64 {
			t.Errorf("%v: got %q, %v, checksum %q", platform, got, err, assetChecksum(got))
		}
	}
}

func TestAssetNameRejectsUnsupportedPlatform(t *testing.T) {
	if _, err := assetName("windows", "arm64"); err == nil {
		t.Fatal("expected an error for an unpinned platform")
	}
}

func TestCacheDirectoryUsesXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/cache")
	got, err := cacheDirectory()
	if err != nil || got != filepath.Join("/cache", "mesail-tools", "tailwindcss-4.3.5") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestVerifyRefusesChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tailwindcss")
	if err := os.WriteFile(path, []byte("not tailwind"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verify(path, assetChecksum("tailwindcss-macos-arm64"))
	if err == nil || !strings.Contains(err.Error(), "refusing to run") {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureTailwindRefusesTamperedCachedBinary(t *testing.T) {
	if _, err := assetName(runtime.GOOS, runtime.GOARCH); err != nil {
		t.Skip(err)
	}
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	name, _ := assetName(runtime.GOOS, runtime.GOARCH)
	path := filepath.Join(cache, "mesail-tools", "tailwindcss-4.3.5", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureTailwind(os.Stderr); err == nil {
		t.Fatal("expected a tampered binary to be refused")
	}
}
