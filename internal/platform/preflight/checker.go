// Package preflight validates dependencies before any production listener opens.
package preflight

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/platform/config"
)

// Check records one safe-to-display dependency result.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// Report is shared by doctor, migrate, readiness, and startup logs.
type Report struct {
	Status           string            `json:"status"`
	FYLOVersion      string            `json:"fyloVersion"`
	FYLOCommit       string            `json:"fyloCommit,omitempty"`
	FYLOTarget       string            `json:"fyloTarget,omitempty"`
	FYLODigest       string            `json:"fyloDigest,omitempty"`
	FYLOCapabilities []string          `json:"fyloCapabilities,omitempty"`
	FYLODependencies map[string]string `json:"fyloDependencies,omitempty"`
	SESAMEVersion    string            `json:"sesameVersion,omitempty"`
	SESAMETarget     string            `json:"sesameTarget,omitempty"`
	SESAMEDigest     string            `json:"sesameDigest,omitempty"`
	SchemaFiles      int               `json:"schemaFiles"`
	FrontendFiles    int               `json:"frontendFiles"`
	FrontendDigest   string            `json:"frontendDigest"`
	Checks           []Check           `json:"checks"`
	FrontendAssets   []string          `json:"-"`
}

type runtimeLock struct {
	SchemaVersion int `json:"schemaVersion"`
	Artifacts     struct {
		FYLO struct {
			ReleasedVersion           string           `json:"releasedVersion"`
			ReleaseCommit             string           `json:"releaseCommit"`
			RequiredCapabilities      []string         `json:"requiredCapabilities"`
			InstalledReleasedArtifact lockedArtifact   `json:"installedReleasedArtifact"`
			ReleasedArtifacts         []lockedArtifact `json:"releasedArtifacts"`
			DevelopmentPreview        struct {
				RuntimeVersion string `json:"runtimeVersion"`
				SHA256         string `json:"sha256"`
			} `json:"developmentPreview"`
		} `json:"fylo"`
		SESAME struct {
			ReleasedVersion           string           `json:"releasedVersion"`
			ReleaseCommit             string           `json:"releaseCommit"`
			InstalledReleasedArtifact lockedArtifact   `json:"installedReleasedArtifact"`
			ReleasedArtifacts         []lockedArtifact `json:"releasedArtifacts"`
		} `json:"sesame"`
		CHEX struct {
			Version string `json:"version"`
		} `json:"chex"`
		TTID struct {
			Version string `json:"version"`
		} `json:"ttid"`
	} `json:"artifacts"`
}

type lockedArtifact struct {
	Target string `json:"target"`
	SHA256 string `json:"sha256"`
}

type frontendManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Source        string `json:"source"`
	Digest        string `json:"digest"`
	Files         []struct {
		Path   string `json:"path"`
		Bytes  int64  `json:"bytes"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

// Run validates the complete Phase 1 dependency set without starting FYLO.
func Run(ctx context.Context, cfg config.Config) (Report, error) {
	report := Report{Status: "failed"}
	add := func(name, detail string) {
		report.Checks = append(report.Checks, Check{Name: name, Status: "ok", Detail: detail})
	}

	frontendAssets, frontendDigest, err := checkFrontend(cfg.StaticDir)
	if err != nil {
		return report, err
	}
	report.FrontendFiles = len(frontendAssets)
	report.FrontendDigest = frontendDigest
	report.FrontendAssets = frontendAssets
	add("frontend", fmt.Sprintf("%d files", len(frontendAssets)))

	schemaFiles, err := countJSONFiles(cfg.FYLOSchema)
	if err != nil {
		return report, fmt.Errorf("FYLO schema: %w", err)
	}
	if schemaFiles == 0 {
		return report, errors.New("FYLO schema: no JSON schema files found")
	}
	report.SchemaFiles = schemaFiles
	add("schemas", fmt.Sprintf("%d files", schemaFiles))

	if err := checkWritableDirectory(cfg.FYLORoot); err != nil {
		return report, fmt.Errorf("FYLO root: %w", err)
	}
	add("fylo_root", "writable")

	lock, err := readRuntimeLock(cfg.RuntimeLock)
	if err != nil {
		return report, err
	}
	add("runtime_lock", "schema v1")

	actualIdentity, err := verifyFYLOBinary(ctx, cfg, lock)
	if err != nil {
		return report, err
	}
	report.FYLOVersion = actualIdentity.version
	report.FYLOCommit = actualIdentity.commit
	report.FYLOTarget = actualIdentity.target
	report.FYLODigest = actualIdentity.digest
	report.FYLOCapabilities = lock.Artifacts.FYLO.RequiredCapabilities
	report.FYLODependencies = map[string]string{
		"chex": lock.Artifacts.CHEX.Version,
		"ttid": lock.Artifacts.TTID.Version,
	}
	add("fylo_binary", "version "+actualIdentity.version)

	sesameIdentity, err := verifySESAMEBinary(cfg, lock)
	if err != nil {
		return report, err
	}
	report.SESAMEVersion = sesameIdentity.version
	report.SESAMETarget = sesameIdentity.target
	report.SESAMEDigest = sesameIdentity.digest
	add("sesame_binary", "version "+sesameIdentity.version)

	if err := checkSESAMEDeployment(cfg.SESAMEDeployment); err != nil {
		return report, err
	}
	add("sesame_deployment", "initialized")

	report.Status = "ok"
	return report, nil
}

// verifySESAMEBinary proves the installed engine is one of the digests the
// runtime lock pins. An unpinned binary is refused: the authorization boundary
// is the last place to accept "probably the right build".
func verifySESAMEBinary(cfg config.Config, lock runtimeLock) (artifactIdentity, error) {
	locked := lock.Artifacts.SESAME
	if locked.ReleasedVersion == "" {
		return artifactIdentity{}, errors.New("runtime lock: no SESAME release is pinned")
	}
	info, err := os.Stat(cfg.SESAMEBinary)
	if err != nil {
		return artifactIdentity{}, fmt.Errorf("SESAME binary: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return artifactIdentity{}, errors.New("SESAME binary is not an executable regular file")
	}
	_, digest, err := fileDigest(cfg.SESAMEBinary)
	if err != nil {
		return artifactIdentity{}, fmt.Errorf("SESAME binary digest: %w", err)
	}
	for _, artifact := range append(locked.ReleasedArtifacts, locked.InstalledReleasedArtifact) {
		if len(artifact.SHA256) == 64 && artifact.SHA256 == digest {
			return artifactIdentity{
				version: locked.ReleasedVersion,
				commit:  locked.ReleaseCommit,
				target:  artifact.Target,
				digest:  digest,
			}, nil
		}
	}
	return artifactIdentity{}, fmt.Errorf("SESAME binary digest %s is not present in runtime lock", digest)
}

type artifactIdentity struct {
	version string
	commit  string
	target  string
	digest  string
}

// checkSESAMEDeployment refuses to start against a directory that has not been
// initialized. Creating the key boundary implicitly here would silently mint a
// new signing key on a mounted volume whose keys failed to appear, and every
// existing session would end without an explanation.
func checkSESAMEDeployment(deployment string) error {
	if strings.TrimSpace(deployment) == "" {
		return errors.New("SESAME deployment: SESAME_DEPLOYMENT is required")
	}
	for _, required := range []string{
		"config.json",
		filepath.Join("keys", "signing.key"),
		filepath.Join("keys", "secrets.key"),
	} {
		path := filepath.Join(deployment, required)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf(
				"SESAME deployment %s is not initialized (%s missing); run `caduceus auth init`",
				deployment, required,
			)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SESAME deployment entry %s is not a regular file", path)
		}
	}
	return nil
}

// RunPersistence validates only the dependencies needed by the private FYLO
// owner, before its bridge listener opens.
func RunPersistence(ctx context.Context, cfg config.Config) (Report, error) {
	report := Report{Status: "failed"}
	if err := checkWritableDirectory(cfg.FYLORoot); err != nil {
		return report, fmt.Errorf("FYLO root: %w", err)
	}
	report.Checks = append(report.Checks, Check{Name: "fylo_root", Status: "ok", Detail: "writable"})
	lock, err := readRuntimeLock(cfg.RuntimeLock)
	if err != nil {
		return report, err
	}
	report.Checks = append(report.Checks, Check{Name: "runtime_lock", Status: "ok", Detail: "schema v1"})
	identity, err := verifyFYLOBinary(ctx, cfg, lock)
	if err != nil {
		return report, err
	}
	report.FYLOVersion = identity.version
	report.FYLOCommit = identity.commit
	report.FYLOTarget = identity.target
	report.FYLODigest = identity.digest
	report.FYLOCapabilities = lock.Artifacts.FYLO.RequiredCapabilities
	report.FYLODependencies = map[string]string{
		"chex": lock.Artifacts.CHEX.Version,
		"ttid": lock.Artifacts.TTID.Version,
	}
	report.Checks = append(report.Checks, Check{
		Name: "fylo_binary", Status: "ok", Detail: "version " + identity.version,
	})
	// The bridge process owns both persistent children, so it qualifies both
	// before it opens a listener.
	sesameIdentity, err := verifySESAMEBinary(cfg, lock)
	if err != nil {
		return report, err
	}
	if err := checkSESAMEDeployment(cfg.SESAMEDeployment); err != nil {
		return report, err
	}
	report.SESAMEVersion = sesameIdentity.version
	report.SESAMETarget = sesameIdentity.target
	report.SESAMEDigest = sesameIdentity.digest
	report.Checks = append(report.Checks, Check{
		Name: "sesame_binary", Status: "ok", Detail: "version " + sesameIdentity.version,
	})
	report.Status = "ok"
	return report, nil
}

func checkFrontend(root string) ([]string, string, error) {
	info, err := os.Stat(filepath.Join(root, "index.html"))
	if err != nil {
		return nil, "", fmt.Errorf("frontend bundle index: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", errors.New("frontend bundle index is not a regular file")
	}
	manifest, err := loadFrontendManifest(root)
	if err != nil {
		return nil, "", err
	}
	return verifyFrontendManifest(root, manifest)
}

func loadFrontendManifest(root string) (frontendManifest, error) {
	data, err := os.ReadFile(filepath.Join(root, ".caduceus-bundle.json"))
	if err != nil {
		return frontendManifest{}, fmt.Errorf("frontend bundle manifest: %w", err)
	}
	var manifest frontendManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return frontendManifest{}, fmt.Errorf("frontend bundle manifest: decode: %w", err)
	}
	if manifest.SchemaVersion != 1 || manifest.Source != "tachyon" || len(manifest.Files) == 0 {
		return frontendManifest{}, errors.New("frontend bundle manifest is unsupported or empty")
	}
	return manifest, nil
}

func verifyFrontendManifest(root string, manifest frontendManifest) ([]string, string, error) {
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	aggregate := sha256.New()
	seen := make(map[string]struct{}, len(manifest.Files))
	assets := make([]string, 0, len(manifest.Files))
	hasIndex := false
	for _, file := range manifest.Files {
		if err := verifyFrontendEntry(root, file, seen); err != nil {
			return nil, "", err
		}
		assets = append(assets, file.Path)
		hasIndex = hasIndex || file.Path == "index.html"
		_, _ = fmt.Fprintf(aggregate, "%s\x00%d\x00%s\n", file.Path, file.Bytes, file.SHA256)
	}
	if !hasIndex {
		return nil, "", errors.New("frontend bundle manifest does not include index.html")
	}
	if fmt.Sprintf("%x", aggregate.Sum(nil)) != manifest.Digest {
		return nil, "", errors.New("frontend bundle aggregate digest differs from manifest")
	}
	return assets, manifest.Digest, nil
}

func verifyFrontendEntry(
	root string,
	file struct {
		Path   string `json:"path"`
		Bytes  int64  `json:"bytes"`
		SHA256 string `json:"sha256"`
	},
	seen map[string]struct{},
) error {
	clean := filepath.ToSlash(filepath.Clean(file.Path))
	if clean != file.Path || clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(file.Path) {
		return fmt.Errorf("frontend bundle manifest contains unsafe path %q", file.Path)
	}
	if _, exists := seen[file.Path]; exists {
		return fmt.Errorf("frontend bundle manifest repeats %q", file.Path)
	}
	seen[file.Path] = struct{}{}
	assetPath := filepath.Join(root, filepath.FromSlash(file.Path))
	if err := rejectSymlinkEscape(root, assetPath); err != nil {
		return fmt.Errorf("frontend asset %q: %w", file.Path, err)
	}
	actualBytes, actualDigest, err := fileDigest(assetPath)
	if err != nil {
		return fmt.Errorf("frontend asset %q: %w", file.Path, err)
	}
	if actualBytes != file.Bytes || actualDigest != file.SHA256 {
		return fmt.Errorf("frontend asset %q differs from manifest", file.Path)
	}
	return nil
}

func rejectSymlinkEscape(root, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symbolic links are forbidden")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path resolves outside the frontend root")
	}
	return nil
}

func fileDigest(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() {
		return 0, "", errors.New("not a regular file")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return 0, "", err
	}
	return info.Size(), fmt.Sprintf("%x", digest.Sum(nil)), nil
}

func countJSONFiles(root string) (int, error) {
	info, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, errors.New("path is not a directory")
	}
	count := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".json") {
			count++
		}
		return nil
	})
	return count, err
}

func checkWritableDirectory(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("path is not a directory")
	}
	probe, err := os.CreateTemp(root, ".caduceus-preflight-*")
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	name := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		_ = os.Remove(name)
		return fmt.Errorf("close write probe: %w", closeErr)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("remove write probe: %w", err)
	}
	return nil
}

func readRuntimeLock(path string) (runtimeLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeLock{}, fmt.Errorf("runtime lock: %w", err)
	}
	var lock runtimeLock
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&lock); err != nil {
		return runtimeLock{}, fmt.Errorf("runtime lock: decode: %w", err)
	}
	if lock.SchemaVersion != 1 || lock.Artifacts.FYLO.ReleasedVersion == "" {
		return runtimeLock{}, errors.New("runtime lock: unsupported or incomplete lock")
	}
	return lock, nil
}

func verifyFYLOBinary(ctx context.Context, cfg config.Config, lock runtimeLock) (fyloIdentity, error) {
	info, err := os.Stat(cfg.FYLOBinary)
	if err != nil {
		return fyloIdentity{}, fmt.Errorf("FYLO binary: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fyloIdentity{}, errors.New("FYLO binary is not an executable regular file")
	}
	_, digest, err := fileDigest(cfg.FYLOBinary)
	if err != nil {
		return fyloIdentity{}, fmt.Errorf("FYLO binary digest: %w", err)
	}
	identities := lockedFYLODigests(lock)
	if len(identities) > 0 {
		identity, known := identities[digest]
		if !known {
			return fyloIdentity{}, fmt.Errorf("FYLO binary digest %s is not present in runtime lock", digest)
		}
		if identity.preview && strings.EqualFold(cfg.NodeEnv, "production") {
			return fyloIdentity{}, errors.New("FYLO preview binary is forbidden in production")
		}
		identity.digest = digest
		return identity, nil
	}
	version, err := legacyFYLOVersion(ctx, cfg.FYLOBinary, lock.Artifacts.FYLO.ReleasedVersion)
	return fyloIdentity{version: version, digest: digest}, err
}

type fyloIdentity struct {
	version string
	commit  string
	target  string
	digest  string
	preview bool
}

func lockedFYLODigests(lock runtimeLock) map[string]fyloIdentity {
	found := make(map[string]fyloIdentity)
	released := lock.Artifacts.FYLO.ReleasedVersion
	for _, artifact := range append(
		lock.Artifacts.FYLO.ReleasedArtifacts,
		lock.Artifacts.FYLO.InstalledReleasedArtifact,
	) {
		if len(artifact.SHA256) == 64 {
			found[artifact.SHA256] = fyloIdentity{
				version: released,
				commit:  lock.Artifacts.FYLO.ReleaseCommit,
				target:  artifact.Target,
			}
		}
	}
	preview := lock.Artifacts.FYLO.DevelopmentPreview
	if len(preview.SHA256) == 64 && preview.RuntimeVersion != "" {
		found[preview.SHA256] = fyloIdentity{version: preview.RuntimeVersion, preview: true}
	}
	return found
}

func legacyFYLOVersion(ctx context.Context, binary, expected string) (string, error) {
	checkContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(checkContext, binary, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("FYLO binary version check: %w", err)
	}
	actual := strings.TrimSpace(string(output))
	if !containsVersion(actual, expected) {
		return "", fmt.Errorf("FYLO binary version %q does not match runtime lock %q", actual, expected)
	}
	return expected, nil
}

func containsVersion(actual, expected string) bool {
	for _, field := range strings.Fields(actual) {
		if strings.TrimPrefix(field, "v") == strings.TrimPrefix(expected, "v") {
			return true
		}
	}
	return false
}
