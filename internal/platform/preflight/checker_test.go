package preflight

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d31ma/caduceus/internal/platform/config"
)

func TestRunAcceptsManifestedRuntime(t *testing.T) {
	cfg := testConfig(t)
	report, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || report.FrontendFiles != 1 || report.SchemaFiles != 1 || report.FYLOVersion != "26.30.04" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.SESAMEVersion != "26.33.02" {
		t.Fatalf("SESAME version = %q", report.SESAMEVersion)
	}
	if len(report.Checks) != 7 {
		t.Fatalf("checks = %d, want 7", len(report.Checks))
	}
}

func TestRunPersistenceChecksOnlyFYLODependencies(t *testing.T) {
	cfg := testConfig(t)
	cfg.StaticDir = filepath.Join(t.TempDir(), "missing-static")

	report, err := RunPersistence(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || report.FYLOVersion != "26.30.04" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Checks) != 4 {
		t.Fatalf("checks = %d, want 4", len(report.Checks))
	}
}

func TestRunPersistenceRejectsAnInvalidFYLODependency(t *testing.T) {
	cfg := testConfig(t)
	cfg.FYLORoot = filepath.Join(t.TempDir(), "missing-root")
	writeFile(t, cfg.FYLORoot, "not a directory", 0o600)

	_, err := RunPersistence(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "FYLO root") {
		t.Fatalf("RunPersistence error = %v", err)
	}
}

func TestRunRejectsDependencyDrift(t *testing.T) {
	tests := []struct {
		name    string
		change  func(t *testing.T, cfg *config.Config)
		message string
	}{
		{
			name: "tampered frontend",
			change: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				writeFile(t, filepath.Join(cfg.StaticDir, "index.html"), "changed", 0o600)
			},
			message: "differs from manifest",
		},
		{
			name: "empty schemas",
			change: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				if err := os.Remove(filepath.Join(cfg.FYLOSchema, "users.json")); err != nil {
					t.Fatal(err)
				}
			},
			message: "no JSON schema",
		},
		{
			name: "root is file",
			change: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				if err := os.RemoveAll(cfg.FYLORoot); err != nil {
					t.Fatal(err)
				}
				writeFile(t, cfg.FYLORoot, "not a directory", 0o600)
			},
			message: "not a directory",
		},
		{
			name: "missing lock",
			change: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				cfg.RuntimeLock = filepath.Join(t.TempDir(), "missing.json")
			},
			message: "runtime lock",
		},
		{
			name: "non executable binary",
			change: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				if err := os.Chmod(cfg.FYLOBinary, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			message: "not an executable",
		},
		{
			name: "wrong binary version",
			change: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				writeFile(t, cfg.FYLOBinary, "#!/bin/sh\nprintf '0.0.0\\n'\n", 0o700)
			},
			message: "does not match",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := testConfig(t)
			test.change(t, &cfg)
			_, err := Run(context.Background(), cfg)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Run error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestFrontendManifestRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	for _, path := range []string{"../index.html", "/index.html"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "index.html"), "shell", 0o600)
			writeManifest(t, root, []manifestFile{{Path: path, Contents: "shell"}})
			_, _, err := checkFrontend(root)
			if err == nil || !strings.Contains(err.Error(), "unsafe path") {
				t.Fatalf("checkFrontend error = %v", err)
			}
		})
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "index.html"), "shell", 0o600)
	writeManifest(t, root, []manifestFile{
		{Path: "index.html", Contents: "shell"},
		{Path: "index.html", Contents: "shell"},
	})
	if _, _, err := checkFrontend(root); err == nil || !strings.Contains(err.Error(), "repeats") {
		t.Fatalf("duplicate manifest error = %v", err)
	}
}

func TestFrontendManifestRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.html")
	writeFile(t, outside, "shell", 0o600)
	if err := os.Symlink(outside, filepath.Join(root, "index.html")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeManifest(t, root, []manifestFile{{Path: "index.html", Contents: "shell"}})
	if _, _, err := checkFrontend(root); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("symlink error = %v", err)
	}
}

func TestContainsVersionRequiresACompleteField(t *testing.T) {
	if !containsVersion("fylo v26.30.04", "26.30.04") {
		t.Fatal("version field was not recognized")
	}
	if containsVersion("fylo 126.30.040", "26.30.04") {
		t.Fatal("substring was accepted as a version")
	}
}

func TestRunUsesLockedDigestWhenVersionCommandIsUnavailable(t *testing.T) {
	cfg := testConfig(t)
	binary := "#!/bin/sh\nexit 7\n"
	writeFile(t, cfg.FYLOBinary, binary, 0o700)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(binary)))
	writeFile(t, cfg.RuntimeLock, lockWithSESAME(t, fmt.Sprintf(
		`"fylo":{"releasedVersion":"26.30.04","releasedArtifacts":[{"target":"test","sha256":"%s"}]}`,
		digest,
	), cfg.SESAMEBinary), 0o600)
	report, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.FYLOVersion != "26.30.04" {
		t.Fatalf("FYLO version = %q", report.FYLOVersion)
	}
}

func TestRunRejectsPreviewDigestInProduction(t *testing.T) {
	cfg := testConfig(t)
	data, err := os.ReadFile(cfg.FYLOBinary)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	writeFile(t, cfg.RuntimeLock, lockWithSESAME(t, fmt.Sprintf(
		`"fylo":{"releasedVersion":"26.30.04","developmentPreview":{"runtimeVersion":"26.30.05-1","sha256":"%s"}}`,
		digest,
	), cfg.SESAMEBinary), 0o600)
	cfg.NodeEnv = "production"
	if _, err := Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("Run error = %v", err)
	}
}

type manifestFile struct {
	Path     string
	Contents string
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	staticRoot := filepath.Join(root, "static")
	schemaRoot := filepath.Join(root, "schemas")
	fyloRoot := filepath.Join(root, "fylo")
	for _, directory := range []string{staticRoot, schemaRoot, fyloRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(staticRoot, "index.html"), "shell", 0o600)
	writeManifest(t, staticRoot, []manifestFile{{Path: "index.html", Contents: "shell"}})
	writeFile(t, filepath.Join(schemaRoot, "users.json"), `{}`, 0o600)
	binary := filepath.Join(root, "fylo-bin")
	writeFile(t, binary, "#!/bin/sh\nprintf 'fylo v26.30.04\\n'\n", 0o700)
	sesameBinary, sesameDeployment := writeSESAME(t, root)
	lock := filepath.Join(root, "runtime.lock.json")
	writeFile(t, lock, lockWithSESAME(t, `"fylo":{"releasedVersion":"26.30.04"}`, sesameBinary), 0o600)

	cfg := config.Defaults()
	cfg.StaticDir = staticRoot
	cfg.FYLOSchema = schemaRoot
	cfg.FYLORoot = fyloRoot
	cfg.FYLOBinary = binary
	cfg.RuntimeLock = lock
	cfg.InboundWebhookSecret = "fedcba9876543210"
	cfg.SESAMEBinary = sesameBinary
	cfg.SESAMEDeployment = sesameDeployment
	cfg.SESAMEIssuer = "https://mail.example.com"
	return cfg
}

// writeSESAME creates a stand-in engine binary and an initialized deployment.
// Preflight verifies the digest and the key boundary's presence, never the
// engine's behaviour, so a file with known bytes is exactly the right fixture.
func writeSESAME(t *testing.T, root string) (string, string) {
	t.Helper()
	binary := filepath.Join(root, "sesame-bin")
	writeFile(t, binary, "#!/bin/sh\nprintf 'sesame 26.33.02\\n'\n", 0o700)
	deployment := filepath.Join(root, "sesame-deployment")
	if err := os.MkdirAll(filepath.Join(deployment, "keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(deployment, "config.json"), `{"config_version":1}`, 0o600)
	writeFile(t, filepath.Join(deployment, "keys", "signing.key"), "signing", 0o600)
	writeFile(t, filepath.Join(deployment, "keys", "secrets.key"), "secrets", 0o600)
	return binary, deployment
}

// lockWithSESAME builds a runtime lock whose SESAME digest matches the fixture
// binary, so every test that rewrites the FYLO half keeps a valid engine pin.
func lockWithSESAME(t *testing.T, fylo, sesameBinary string) string {
	t.Helper()
	data, err := os.ReadFile(sesameBinary)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(
		`{"schemaVersion":1,"artifacts":{%s,"sesame":{"releasedVersion":"26.33.02",`+
			`"releasedArtifacts":[{"target":"test","sha256":"%x"}]}}}`,
		fylo, sha256.Sum256(data),
	)
}

func writeManifest(t *testing.T, root string, files []manifestFile) {
	t.Helper()
	entries := make([]map[string]any, 0, len(files))
	aggregate := sha256.New()
	for _, file := range files {
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(file.Contents)))
		entries = append(entries, map[string]any{
			"path": file.Path, "bytes": len(file.Contents), "sha256": sum,
		})
		_, _ = fmt.Fprintf(aggregate, "%s\x00%d\x00%s\n", file.Path, len(file.Contents), sum)
	}
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"source":        "tachyon",
		"files":         entries,
		"digest":        fmt.Sprintf("%x", aggregate.Sum(nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".caduceus-bundle.json"), string(data), 0o600)
}

func writeFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}
