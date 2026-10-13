// Package fyloaudit reconciles attachment metadata with FYLO bucket objects.
package fyloaudit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/d31ma/caduceus/internal/platform/fylo"
)

// Owner is the minimum single-writer surface required for reconciliation.
type Owner interface {
	Call(context.Context, map[string]any) (fylo.Response, error)
	WithQuiescedRoot(context.Context, func(string) error) error
}

// AttachmentReport describes drift without mutating it.
type AttachmentReport struct {
	Referenced int      `json:"referenced"`
	Bucket     int      `json:"bucket"`
	Missing    []string `json:"missing"`
	Orphans    []string `json:"orphans"`
	Corrupt    []string `json:"corrupt"`
	Applied    bool     `json:"applied"`
}

// Reconcile reports drift and deletes only orphan bucket objects when apply is
// explicitly true. Missing metadata references are never silently rewritten.
func Reconcile(ctx context.Context, owner Owner, apply bool) (AttachmentReport, error) {
	if owner == nil {
		return AttachmentReport{}, errors.New("FYLO attachment audit requires an owner")
	}
	metadata, err := call(ctx, owner, map[string]any{
		"op": "findDocs", "collection": "attachments", "query": map[string]any{"$ops": []any{}},
	})
	if err != nil {
		return AttachmentReport{}, fmt.Errorf("read attachment metadata: %w", err)
	}
	referenced, err := referencedBucketIDs(metadata.Result)
	if err != nil {
		return AttachmentReport{}, err
	}
	verify, err := call(ctx, owner, map[string]any{
		"op": "verifyCollection", "collection": "attachment-files",
	})
	if err != nil {
		return AttachmentReport{}, fmt.Errorf("verify attachment bucket: %w", err)
	}
	corrupt := verifyCorruptIDs(verify.Result)
	bucket := map[string]struct{}{}
	err = owner.WithQuiescedRoot(ctx, func(root string) error {
		var walkErr error
		bucket, walkErr = bucketIDs(filepath.Join(
			root, ".buckets", "attachment-files", "docs",
		))
		return walkErr
	})
	if err != nil {
		return AttachmentReport{}, err
	}
	report := AttachmentReport{
		Referenced: len(referenced),
		Bucket:     len(bucket),
		Corrupt:    corrupt,
	}
	for id := range referenced {
		if _, found := bucket[id]; !found {
			report.Missing = append(report.Missing, id)
		}
	}
	for id := range bucket {
		if _, found := referenced[id]; !found {
			report.Orphans = append(report.Orphans, id)
		}
	}
	sort.Strings(report.Missing)
	sort.Strings(report.Orphans)
	if apply {
		for _, id := range report.Orphans {
			if _, err := call(ctx, owner, map[string]any{
				"op": "delDoc", "collection": "attachment-files", "id": id,
			}); err != nil {
				return report, fmt.Errorf("delete orphan bucket object %s: %w", id, err)
			}
		}
		report.Applied = true
	}
	return report, nil
}

func referencedBucketIDs(raw json.RawMessage) (map[string]struct{}, error) {
	var documents map[string]map[string]any
	if err := json.Unmarshal(raw, &documents); err != nil {
		return nil, fmt.Errorf("decode attachment metadata: %w", err)
	}
	ids := map[string]struct{}{}
	for _, document := range documents {
		if id, ok := document["bucketId"].(string); ok && id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids, nil
}

func verifyCorruptIDs(raw json.RawMessage) []string {
	var result struct {
		Corrupt []any `json:"corrupt"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return []string{"unknown"}
	}
	ids := make([]string, 0, len(result.Corrupt))
	for _, value := range result.Corrupt {
		switch item := value.(type) {
		case string:
			ids = append(ids, item)
		case map[string]any:
			if id, ok := item["id"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func bucketIDs(root string) (map[string]struct{}, error) {
	ids := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("attachment bucket contains symbolic link %s", path)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		name := entry.Name()
		index := strings.IndexByte(name, '.')
		if index <= 0 {
			return fmt.Errorf("attachment bucket contains malformed object %s", path)
		}
		ids[name[:index]] = struct{}{}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return ids, nil
	}
	return ids, err
}

func call(ctx context.Context, owner Owner, request map[string]any) (fylo.Response, error) {
	response, err := owner.Call(ctx, request)
	if err != nil {
		return fylo.Response{}, err
	}
	if !response.OK {
		if response.Error != nil {
			return fylo.Response{}, response.Error
		}
		return fylo.Response{}, errors.New("FYLO audit operation failed")
	}
	return response, nil
}
