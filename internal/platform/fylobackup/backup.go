// Package fylobackup creates and restores verified local FYLO snapshots without
// requiring object storage or another external service.
package fylobackup

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const ManifestName = "manifest.json"

// Quiescer holds FYLO's root lease while no request can mutate the filesystem.
type Quiescer interface {
	WithQuiescedRoot(context.Context, func(string) error) error
}

// Manifest is the portable, digest-bound local backup contract.
type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	SourceDigest  string    `json:"sourceDigest"`
	Entries       []Entry   `json:"entries"`
}

// Entry records one relative root object.
type Entry struct {
	Path       string            `json:"path"`
	Type       string            `json:"type"`
	Size       int64             `json:"size"`
	Mode       uint32            `json:"mode"`
	SHA256     string            `json:"sha256,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Create takes a quiesced, lease-protected local snapshot.
func Create(ctx context.Context, owner Quiescer, destination string) (Manifest, error) {
	var manifest Manifest
	if owner == nil {
		return manifest, errors.New("FYLO backup requires a root quiescer")
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return manifest, fmt.Errorf("resolve backup destination: %w", err)
	}
	if _, err := os.Lstat(absolute); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return manifest, errors.New("backup destination already exists")
		}
		return manifest, err
	}
	err = owner.WithQuiescedRoot(ctx, func(root string) error {
		if err := os.Mkdir(absolute, 0o700); err != nil {
			return err
		}
		dataRoot := filepath.Join(absolute, "data")
		if err := os.Mkdir(dataRoot, 0o700); err != nil {
			return err
		}
		entries, err := copyTree(root, dataRoot)
		if err != nil {
			return err
		}
		manifest = Manifest{
			SchemaVersion: 1,
			CreatedAt:     time.Now().UTC(),
			Entries:       entries,
			SourceDigest:  aggregateDigest(entries),
		}
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		return writeExclusive(filepath.Join(absolute, ManifestName), encoded, 0o600)
	})
	if err != nil {
		_ = os.RemoveAll(absolute)
		return Manifest{}, fmt.Errorf("create FYLO backup: %w", err)
	}
	return manifest, nil
}

// Verify re-hashes every backup object and required FYLO extended attribute.
func Verify(ctx context.Context, backup string) (Manifest, error) {
	absolute, err := filepath.Abs(backup)
	if err != nil {
		return Manifest{}, err
	}
	manifest, err := readManifest(filepath.Join(absolute, ManifestName))
	if err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != 1 || manifest.SourceDigest != aggregateDigest(manifest.Entries) {
		return Manifest{}, errors.New("FYLO backup manifest digest is invalid")
	}
	dataRoot := filepath.Join(absolute, "data")
	for _, entry := range manifest.Entries {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		path, err := safeJoin(dataRoot, entry.Path)
		if err != nil {
			return Manifest{}, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return Manifest{}, fmt.Errorf("verify %s: %w", entry.Path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return Manifest{}, fmt.Errorf("verify %s: symbolic links are forbidden", entry.Path)
		}
		if uint32(info.Mode().Perm()) != entry.Mode {
			return Manifest{}, fmt.Errorf("verify %s: mode differs", entry.Path)
		}
		switch entry.Type {
		case "directory":
			if !info.IsDir() {
				return Manifest{}, fmt.Errorf("verify %s: expected directory", entry.Path)
			}
		case "file":
			if !info.Mode().IsRegular() || info.Size() != entry.Size {
				return Manifest{}, fmt.Errorf("verify %s: file metadata differs", entry.Path)
			}
			digest, err := digestFile(path)
			if err != nil || digest != entry.SHA256 {
				return Manifest{}, fmt.Errorf("verify %s: SHA-256 differs", entry.Path)
			}
		default:
			return Manifest{}, fmt.Errorf("verify %s: unsupported entry type", entry.Path)
		}
		attributes, err := readFYLOAttributes(path)
		if err != nil || !sameAttributes(attributes, entry.Attributes) {
			return Manifest{}, fmt.Errorf("verify %s: FYLO attributes differ", entry.Path)
		}
	}
	return manifest, nil
}

// Restore verifies a backup before copying it to a new destination.
func Restore(ctx context.Context, backup, destination string) (Manifest, error) {
	manifest, err := Verify(ctx, backup)
	if err != nil {
		return Manifest{}, err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return Manifest{}, err
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return Manifest{}, errors.New("restore destination already exists")
		}
		return Manifest{}, err
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		return Manifest{}, err
	}
	sourceRoot, _ := filepath.Abs(filepath.Join(backup, "data"))
	for _, entry := range manifest.Entries {
		if err := ctx.Err(); err != nil {
			_ = os.RemoveAll(destination)
			return Manifest{}, err
		}
		source, err := safeJoin(sourceRoot, entry.Path)
		if err != nil {
			_ = os.RemoveAll(destination)
			return Manifest{}, err
		}
		target, err := safeJoin(destination, entry.Path)
		if err != nil {
			_ = os.RemoveAll(destination)
			return Manifest{}, err
		}
		if entry.Type == "directory" {
			err = os.MkdirAll(target, fs.FileMode(entry.Mode))
		} else {
			err = copyFile(source, target, fs.FileMode(entry.Mode))
		}
		if err == nil {
			err = writeFYLOAttributes(target, entry.Attributes)
		}
		if err != nil {
			_ = os.RemoveAll(destination)
			return Manifest{}, fmt.Errorf("restore %s: %w", entry.Path, err)
		}
	}
	return manifest, nil
}

func copyTree(sourceRoot, destinationRoot string) ([]Entry, error) {
	var entries []Entry
	err := filepath.WalkDir(sourceRoot, func(source string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if source == sourceRoot {
			return nil
		}
		relative, err := filepath.Rel(sourceRoot, source)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if excludedRuntimePath(relative) {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: symbolic links are forbidden", relative)
		}
		target, err := safeJoin(destinationRoot, relative)
		if err != nil {
			return err
		}
		entry := Entry{Path: relative, Mode: uint32(info.Mode().Perm())}
		if item.IsDir() {
			entry.Type = "directory"
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			entry.Type = "file"
			entry.Size = info.Size()
			if err := copyFile(source, target, info.Mode().Perm()); err != nil {
				return err
			}
			entry.SHA256, err = digestFile(source)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("%s: unsupported filesystem object", relative)
		}
		entry.Attributes, err = readFYLOAttributes(source)
		if err != nil {
			return err
		}
		if err := writeFYLOAttributes(target, entry.Attributes); err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Path < entries[right].Path })
	return entries, nil
}

func copyFile(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	return errors.Join(copyErr, closeErr)
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func aggregateDigest(entries []Entry) string {
	digest := sha256.New()
	for _, entry := range entries {
		attributes, _ := json.Marshal(entry.Attributes)
		_, _ = fmt.Fprintf(
			digest, "%s\x00%s\x00%d\x00%d\x00%s\x00%s\n",
			entry.Path, entry.Type, entry.Size, entry.Mode, entry.SHA256, attributes,
		)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func readManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read FYLO backup manifest: %w", err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode FYLO backup manifest: %w", err)
	}
	return manifest, nil
}

func safeJoin(root, relative string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(relative))
	if clean != relative || clean == "." || filepath.IsAbs(relative) ||
		strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe FYLO backup path %q", relative)
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	within, err := filepath.Rel(root, target)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("FYLO backup path escapes root: %q", relative)
	}
	return target, nil
}

func excludedRuntimePath(relative string) bool {
	first := strings.Split(relative, "/")[0]
	return first == ".caduceus-incoming" || first == ".fylo-root.lock"
}

func writeExclusive(path string, data []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	return errors.Join(writeErr, closeErr)
}

func sameAttributes(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for name, value := range left {
		if right[name] != value {
			return false
		}
	}
	return true
}

func encodeAttribute(value []byte) string {
	return base64.RawStdEncoding.EncodeToString(value)
}

func decodeAttribute(value string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(value)
}
