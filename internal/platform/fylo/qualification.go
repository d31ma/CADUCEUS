package fylo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// QualifyRelease actively proves the FYLO operations that are not completely
// described by the release handshake.
func QualifyRelease(ctx context.Context, options QualificationOptions) (QualificationReport, error) {
	root, err := os.MkdirTemp("", "caduceus-fylo-qualification-*")
	if err != nil {
		return QualificationReport{}, fmt.Errorf("create qualification root: %w", err)
	}
	defer os.RemoveAll(root)

	processOptions := Options{
		Binary:           options.Binary,
		Root:             root,
		ExpectedVersion:  options.ExpectedVersion,
		MaxRequestBytes:  options.MaxRequestBytes,
		MaxResponseBytes: options.MaxResponseBytes,
		RequestTimeout:   10 * time.Second,
		ShutdownTimeout:  10 * time.Second,
		RestartLimit:     4,
	}
	owner := New(processOptions)
	if err := owner.Start(ctx); err != nil {
		return QualificationReport{}, fmt.Errorf("start released FYLO: %w", err)
	}
	defer func() { _ = owner.Close(context.Background()) }()

	checks := map[string]bool{}
	contender := New(processOptions)
	if err := contender.Start(ctx); !HasCode(err, "EROOTLOCKED") {
		return QualificationReport{}, fmt.Errorf("exclusive-root probe: got %v", err)
	}
	checks["exclusiveRoot"] = true

	if _, err := callSuccess(ctx, owner, map[string]any{
		"op": "createCollection", "collection": "messages", "kind": "document",
	}); err != nil {
		return QualificationReport{}, err
	}
	if _, err := callSuccess(ctx, owner, map[string]any{
		"op": "createCollection", "collection": "attachment-files", "kind": "file",
	}); err != nil {
		return QualificationReport{}, err
	}

	uid, gid := currentAccessIDs()
	securePut, err := callSuccess(ctx, owner, map[string]any{
		"op":         "putData",
		"collection": "messages",
		"data":       map[string]any{"domain": "phase2.example", "subject": "tenant-visible"},
		"access":     map[string]any{"uid": uid, "gid": gid, "mode": 0o640},
	})
	if err != nil {
		return QualificationReport{}, fmt.Errorf("machine access write probe: %w", err)
	}
	var secureID string
	if err := json.Unmarshal(securePut.Result, &secureID); err != nil {
		return QualificationReport{}, fmt.Errorf("decode access probe identifier: %w", err)
	}
	member, err := callSuccess(ctx, owner, map[string]any{
		"op":         "getDoc",
		"collection": "messages",
		"id":         secureID,
		"access":     map[string]any{"uid": uid + 10_000, "groups": []int{gid}},
	})
	if err != nil || !containsJSONText(member.Result, "tenant-visible") {
		return QualificationReport{}, errors.New("trusted virtual group could not read tenant document")
	}
	outsider, err := owner.Call(ctx, map[string]any{
		"op":         "getDoc",
		"collection": "messages",
		"id":         secureID,
		"access":     map[string]any{"uid": uid + 20_000, "groups": []int{}},
	})
	if err != nil {
		return QualificationReport{}, fmt.Errorf("outsider access transport: %w", err)
	}
	if outsider.OK || outsider.Error == nil || outsider.Error.Code != "EACCES" {
		return QualificationReport{}, fmt.Errorf("outsider access was not denied: %+v", outsider)
	}
	checks["machineAccess"] = true

	incoming := filepath.Join(root, ".qualification-incoming")
	if err := os.MkdirAll(incoming, 0o700); err != nil {
		return QualificationReport{}, err
	}
	source := filepath.Join(incoming, "attachment.bin")
	attachmentBytes := []byte("caduceus-phase2-attachment-integrity")
	if err := os.WriteFile(source, attachmentBytes, 0o600); err != nil {
		return QualificationReport{}, err
	}
	filePut, err := callSuccess(ctx, owner, map[string]any{
		"op":         "putData",
		"collection": "attachment-files",
		"file":       map[string]any{"path": source},
		"fileOptions": map[string]any{
			"maxBytes": options.MaxAttachmentBytes,
		},
		"meta":   map[string]any{"domain": "phase2.example"},
		"access": map[string]any{"uid": uid, "gid": gid, "mode": 0o640},
	})
	if err != nil {
		return QualificationReport{}, fmt.Errorf("document bucket write probe: %w", err)
	}
	var attachmentID string
	if err := json.Unmarshal(filePut.Result, &attachmentID); err != nil {
		return QualificationReport{}, fmt.Errorf("decode bucket identifier: %w", err)
	}
	attachment, err := callSuccess(ctx, owner, map[string]any{
		"op":         "getDoc",
		"collection": "attachment-files",
		"id":         attachmentID,
		"access":     map[string]any{"uid": uid + 10_000, "groups": []int{gid}},
	})
	if err != nil || len(attachment.Result) == 0 {
		return QualificationReport{}, errors.New("document bucket metadata could not be read")
	}
	checks["documentBuckets"] = true

	verify, err := callSuccess(ctx, owner, map[string]any{
		"op": "verifyCollection", "collection": "attachment-files",
	})
	if err != nil || resultArrayLength(verify.Result, "corrupt") != 0 {
		return QualificationReport{}, errors.New("clean bucket did not verify")
	}
	// The shard directory under docs/ is FYLO's own business and has changed
	// across releases, so the object is located rather than computed. Qualification is
	// about whether changed bytes are detected, not about where they live.
	bucketPath, err := locateBucketObject(
		filepath.Join(root, ".buckets", "attachment-files", "docs"), attachmentID,
	)
	if err != nil {
		return QualificationReport{}, fmt.Errorf("inject bucket corruption: %w", err)
	}
	if err := os.WriteFile(bucketPath, []byte("corrupted"), 0o640); err != nil {
		return QualificationReport{}, fmt.Errorf("inject bucket corruption: %w", err)
	}
	verify, err = callSuccess(ctx, owner, map[string]any{
		"op": "verifyCollection", "collection": "attachment-files",
	})
	if err != nil || resultArrayLength(verify.Result, "corrupt") == 0 {
		return QualificationReport{}, errors.New("changed attachment bytes were not detected")
	}
	checks["bucketIntegrity"] = true

	if err := owner.Restart(ctx); err != nil {
		return QualificationReport{}, fmt.Errorf("forced restart probe: %w", err)
	}
	persisted, err := callSuccess(ctx, owner, map[string]any{
		"op": "getDoc", "collection": "messages", "id": secureID,
		"access": map[string]any{"uid": uid},
	})
	if err != nil || !containsJSONText(persisted.Result, "tenant-visible") {
		return QualificationReport{}, errors.New("acknowledged write was lost across restart")
	}
	checks["crashTakeover"] = true

	if err := owner.Close(ctx); err != nil {
		return QualificationReport{}, fmt.Errorf("close before index rebuild: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".collections", "messages", "index")); err != nil {
		return QualificationReport{}, fmt.Errorf("remove qualification index: %w", err)
	}
	owner = New(processOptions)
	if err := owner.Start(ctx); err != nil {
		return QualificationReport{}, fmt.Errorf("restart after index loss: %w", err)
	}
	if _, err := callSuccess(ctx, owner, map[string]any{
		"op": "rebuildCollection", "collection": "messages",
	}); err != nil {
		return QualificationReport{}, fmt.Errorf("rebuild collection: %w", err)
	}
	persisted, err = callSuccess(ctx, owner, map[string]any{
		"op": "getDoc", "collection": "messages", "id": secureID,
		"access": map[string]any{"uid": uid},
	})
	if err != nil || !containsJSONText(persisted.Result, "tenant-visible") {
		return QualificationReport{}, errors.New("index rebuild did not recover document truth")
	}
	checks["indexRebuild"] = true

	identity := owner.Identity()
	return QualificationReport{
		SchemaVersion:   1,
		RuntimeVersion:  identity.RuntimeVersion,
		ProtocolVersion: identity.ProtocolVersion,
		ReleaseCommit:   identity.Commit,
		BuildTarget:     identity.BuildTarget,
		Checks:          checks,
		Limits: Limits{
			MaxRequestBytes:      options.MaxRequestBytes,
			MaxResponseBytes:     options.MaxResponseBytes,
			MaxAttachmentBytes:   options.MaxAttachmentBytes,
			AttachmentQuotaBytes: options.AttachmentQuotaBytes,
		},
	}, nil
}

func callSuccess(
	ctx context.Context,
	owner *Supervisor,
	request map[string]any,
) (Response, error) {
	response, err := owner.Call(ctx, request)
	if err != nil {
		return Response{}, err
	}
	if !response.OK {
		if response.Error != nil {
			return Response{}, response.Error
		}
		return Response{}, errors.New("FYLO operation failed without a stable error")
	}
	return response, nil
}

func containsJSONText(raw json.RawMessage, text string) bool {
	return len(raw) > 0 && bytesContains(raw, []byte(text))
}

func bytesContains(haystack, needle []byte) bool {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		if string(haystack[index:index+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

// locateBucketObject finds one bucket object under the docs tree by name.
func locateBucketObject(docs, id string) (string, error) {
	target := id + ".bin"
	found := ""
	err := filepath.WalkDir(docs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == target {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("bucket object %s is not present under %s", target, docs)
	}
	return found, nil
}

func resultArrayLength(raw json.RawMessage, field string) int {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil {
		return -1
	}
	var items []json.RawMessage
	if json.Unmarshal(value[field], &items) != nil {
		return -1
	}
	return len(items)
}
