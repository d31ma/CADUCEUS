package blackbox_test

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/fylo"
	"github.com/d31ma/caduceus/internal/platform/fyloaudit"
	"github.com/d31ma/caduceus/internal/platform/fylobackup"
	"github.com/d31ma/caduceus/internal/platform/fylobridge"
)

func TestPhase2SingleWriterBridgeAndRestart(t *testing.T) {
	binary := releasedFYLO(t)
	root := t.TempDir()
	options := fylo.Options{
		Binary:           binary,
		Root:             root,
		ExpectedVersion:  lockedFYLOVersion,
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 8 << 20,
		RequestTimeout:   5 * time.Second,
		ShutdownTimeout:  5 * time.Second,
		RestartLimit:     3,
	}
	owner := fylo.New(options)
	requireNoError(t, owner.Start(context.Background()))
	t.Cleanup(func() { _ = owner.Close(context.Background()) })

	contender := fylo.New(options)
	err := contender.Start(context.Background())
	if !fylo.HasCode(err, "EROOTLOCKED") {
		t.Fatalf("competing owner error = %v, want EROOTLOCKED", err)
	}

	callOK(t, owner, map[string]any{
		"op": "createCollection", "collection": "messages", "kind": "document",
	})
	put := callOK(t, owner, map[string]any{
		"op":         "putData",
		"collection": "messages",
		"data":       map[string]any{"domain": "example.com", "subject": "durable"},
	})
	var id string
	requireNoError(t, json.Unmarshal(put.Result, &id))

	requireNoError(t, owner.Restart(context.Background()))
	get := callOK(t, owner, map[string]any{
		"op": "getDoc", "collection": "messages", "id": id,
	})
	if !strings.Contains(string(get.Result), "durable") {
		t.Fatalf("acknowledged document was lost after restart: %s", get.Result)
	}

	secret := "phase2-blackbox-bridge-secret-0001"
	bridge := fylobridge.New(fylobridge.Options{
		Address:              "127.0.0.1:0",
		Secret:               secret,
		MaxFrameBytes:        2 << 20,
		RequestTimeout:       5 * time.Second,
		MaxAttachmentBytes:   50 << 20,
		AttachmentQuotaBytes: 10 << 30,
		IncomingRoot:         filepath.Join(root, ".caduceus-incoming"),
	}, owner)
	listener, err := bridge.Listen()
	requireNoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	bridgeContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = bridge.Serve(bridgeContext, listener) }()

	payload, err := json.Marshal(map[string]any{"op": "handshake"})
	requireNoError(t, err)
	response := bridgeRequest(t, listener.Addr().String(), secret, payload, "phase2nonce00000000000001")
	if !response.OK {
		t.Fatalf("authenticated bridge request failed: %+v", response.Error)
	}

	replayed := bridgeRequest(t, listener.Addr().String(), secret, payload, "phase2nonce00000000000001")
	if replayed.Error == nil || replayed.Error.Code != "EREPLAY" {
		t.Fatalf("replayed bridge frame = %+v, want EREPLAY", replayed)
	}

	outside := filepath.Join(t.TempDir(), "outside.bin")
	requireNoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
	forbiddenPayload, err := json.Marshal(map[string]any{
		"op":         "putData",
		"collection": "attachment-files",
		"file":       map[string]any{"path": outside},
	})
	requireNoError(t, err)
	forbidden := bridgeRequest(
		t, listener.Addr().String(), secret, forbiddenPayload, "phase2nonce00000000000002",
	)
	if forbidden.Error == nil || forbidden.Error.Code != "EFORBIDDEN" {
		t.Fatalf("outside bridge path = %+v, want EFORBIDDEN", forbidden)
	}
}

func TestPhase2LegacyRuntimeUsesGoOwnedBridge(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun is required for the legacy bridge qualification")
	}
	binary := releasedFYLO(t)
	root := t.TempDir()
	owner := fylo.New(fylo.Options{
		Binary: binary, Root: root, ExpectedVersion: lockedFYLOVersion,
		RequestTimeout: 10 * time.Second, ShutdownTimeout: 5 * time.Second,
	})
	requireNoError(t, owner.Start(context.Background()))
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	secret := "phase2-legacy-bridge-secret-00001"
	bridge := fylobridge.New(fylobridge.Options{
		Address: "127.0.0.1:0", Secret: secret, MaxFrameBytes: 2 << 20,
		RequestTimeout: 10 * time.Second, MaxAttachmentBytes: 50 << 20,
		AttachmentQuotaBytes: 10 << 30,
		IncomingRoot:         filepath.Join(root, ".caduceus-incoming"),
		DataRoot:             root,
	}, owner)
	listener, err := bridge.Listen()
	requireNoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	bridgeContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = bridge.Serve(bridgeContext, listener) }()

	workingDirectory, err := os.Getwd()
	requireNoError(t, err)
	repository := filepath.Clean(filepath.Join(workingDirectory, "..", ".."))
	module := filepath.Join(repository, "server", "platform", "persistence", "fylo.js")
	script := fmt.Sprintf(
		`import { pathToFileURL } from "node:url";
		 const { createDb, Collections } = await import(pathToFileURL(%q).href);
		 const db = await createDb();
		 const id = await db.putData(Collections.USERS, {email:"bridge@phase2.example",domains:["phase2.example"]});
		 const doc = await db.getDoc(Collections.USERS, id);
		 await db.close();
		 if (!JSON.stringify(doc).includes("bridge@phase2.example")) process.exit(3);
		 process.exit(0);`,
		module,
	)
	commandContext, commandCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer commandCancel()
	command := exec.CommandContext(commandContext, "bun", "-e", script)
	command.Dir = repository
	command.Env = append(os.Environ(),
		"NODE_ENV=production",
		"FYLO_ROOT="+root,
		"FYLO_BRIDGE_ADDRESS="+listener.Addr().String(),
		"FYLO_BRIDGE_SECRET="+secret,
		"FYLO_POSIX_ACCESS=false",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("legacy bridge client failed: %v\n%s", err, output)
	}
	if owner.Stats().Starts != 1 {
		t.Fatalf("legacy request started another FYLO owner: %+v", owner.Stats())
	}
}

func TestPhase2ReleasedFYLOQualification(t *testing.T) {
	report, err := fylo.QualifyRelease(context.Background(), fylo.QualificationOptions{
		Binary:               releasedFYLO(t),
		ExpectedVersion:      lockedFYLOVersion,
		MaxRequestBytes:      1 << 20,
		MaxResponseBytes:     8 << 20,
		MaxAttachmentBytes:   50 << 20,
		AttachmentQuotaBytes: 10 << 30,
	})
	requireNoError(t, err)
	for _, check := range []string{
		"exclusiveRoot",
		"machineAccess",
		"documentBuckets",
		"bucketIntegrity",
		"indexRebuild",
		"crashTakeover",
	} {
		if !report.Checks[check] {
			t.Errorf("qualification check %s did not pass", check)
		}
	}
}

func TestPhase2ColdRestorePreservesTenantAndAttachmentIntegrity(t *testing.T) {
	binary := releasedFYLO(t)
	root := t.TempDir()
	options := fylo.Options{
		Binary:           binary,
		Root:             root,
		ExpectedVersion:  lockedFYLOVersion,
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 8 << 20,
		RequestTimeout:   5 * time.Second,
		ShutdownTimeout:  5 * time.Second,
		RestartLimit:     3,
	}
	owner := fylo.New(options)
	requireNoError(t, owner.Start(context.Background()))
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	callOK(t, owner, map[string]any{
		"op": "createCollection", "collection": "messages", "kind": "document",
	})
	callOK(t, owner, map[string]any{
		"op": "createCollection", "collection": "attachment-files", "kind": "file",
	})
	uid, gid := os.Getuid(), os.Getgid()
	document := callOK(t, owner, map[string]any{
		"op":         "putData",
		"collection": "messages",
		"data":       map[string]any{"domain": "restored.example", "subject": "restored tenant"},
		"access":     map[string]any{"uid": uid, "gid": gid, "mode": 0o640},
	})
	var documentID string
	requireNoError(t, json.Unmarshal(document.Result, &documentID))
	incoming := filepath.Join(root, ".caduceus-incoming")
	requireNoError(t, os.MkdirAll(incoming, 0o700))
	attachmentPath := filepath.Join(incoming, "attachment.bin")
	requireNoError(t, os.WriteFile(attachmentPath, []byte("restore-integrity"), 0o600))
	attachment := callOK(t, owner, map[string]any{
		"op":         "putData",
		"collection": "attachment-files",
		"file":       map[string]any{"path": attachmentPath},
		"access":     map[string]any{"uid": uid, "gid": gid, "mode": 0o640},
	})
	var attachmentID string
	requireNoError(t, json.Unmarshal(attachment.Result, &attachmentID))

	backup := filepath.Join(t.TempDir(), "backup")
	manifest, err := fylobackup.Create(context.Background(), owner, backup)
	requireNoError(t, err)
	if len(manifest.Entries) == 0 {
		t.Fatal("cold backup is empty")
	}
	_, err = fylobackup.Verify(context.Background(), backup)
	requireNoError(t, err)
	restore := filepath.Join(t.TempDir(), "restore")
	_, err = fylobackup.Restore(context.Background(), backup, restore)
	requireNoError(t, err)

	restoredOptions := options
	restoredOptions.Root = restore
	restored := fylo.New(restoredOptions)
	requireNoError(t, restored.Start(context.Background()))
	t.Cleanup(func() { _ = restored.Close(context.Background()) })
	member := callOK(t, restored, map[string]any{
		"op":         "getDoc",
		"collection": "messages",
		"id":         documentID,
		"access":     map[string]any{"uid": uid + 10_000, "groups": []int{gid}},
	})
	if !strings.Contains(string(member.Result), "restored tenant") {
		t.Fatalf("restored tenant decision lost: %s", member.Result)
	}
	outsider, err := restored.Call(context.Background(), map[string]any{
		"op":         "getDoc",
		"collection": "messages",
		"id":         documentID,
		"access":     map[string]any{"uid": uid + 20_000, "groups": []int{}},
	})
	requireNoError(t, err)
	if outsider.OK || outsider.Error == nil || outsider.Error.Code != "EACCES" {
		t.Fatalf("restored outsider decision = %+v, want EACCES", outsider)
	}
	verify := callOK(t, restored, map[string]any{
		"op": "verifyCollection", "collection": "attachment-files",
	})
	if !strings.Contains(string(verify.Result), `"corrupt":[]`) {
		t.Fatalf("restored attachment failed integrity: %s", verify.Result)
	}
	restoredAttachment := callOK(t, restored, map[string]any{
		"op":         "getDoc",
		"collection": "attachment-files",
		"id":         attachmentID,
		"access":     map[string]any{"uid": uid},
	})
	if len(restoredAttachment.Result) == 0 {
		t.Fatal("restored attachment metadata is empty")
	}
}

func TestPhase2AttachmentReconciliationIsDryRunByDefault(t *testing.T) {
	binary := releasedFYLO(t)
	root := t.TempDir()
	owner := fylo.New(fylo.Options{
		Binary: binary, Root: root, ExpectedVersion: lockedFYLOVersion,
		RequestTimeout: 5 * time.Second, ShutdownTimeout: 5 * time.Second,
	})
	requireNoError(t, owner.Start(context.Background()))
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	callOK(t, owner, map[string]any{
		"op": "createCollection", "collection": "attachments", "kind": "document",
	})
	callOK(t, owner, map[string]any{
		"op": "createCollection", "collection": "attachment-files", "kind": "file",
	})
	source := filepath.Join(root, "orphan.bin")
	requireNoError(t, os.WriteFile(source, []byte("orphan"), 0o600))
	orphan := callOK(t, owner, map[string]any{
		"op": "putData", "collection": "attachment-files",
		"file": map[string]any{"path": source},
	})
	var orphanID string
	requireNoError(t, json.Unmarshal(orphan.Result, &orphanID))
	callOK(t, owner, map[string]any{
		"op": "putData", "collection": "attachments",
		"data": map[string]any{"id": "missing", "bucketId": "missing-bucket-id"},
	})

	report, err := fyloaudit.Reconcile(context.Background(), owner, false)
	requireNoError(t, err)
	if report.Applied || !contains(report.Orphans, orphanID) ||
		!contains(report.Missing, "missing-bucket-id") {
		t.Fatalf("dry-run reconciliation = %+v", report)
	}
	callOK(t, owner, map[string]any{
		"op": "getDoc", "collection": "attachment-files", "id": orphanID,
	})
	report, err = fyloaudit.Reconcile(context.Background(), owner, true)
	requireNoError(t, err)
	if !report.Applied {
		t.Fatalf("apply reconciliation = %+v", report)
	}
}

func TestPhase2TransactionalCancellationAndBoundedMixedLoad(t *testing.T) {
	owner := fylo.New(fylo.Options{
		Binary: releasedFYLO(t), Root: t.TempDir(), ExpectedVersion: lockedFYLOVersion,
		RequestTimeout: 10 * time.Second, ShutdownTimeout: 5 * time.Second,
		RestartLimit: 3,
	})
	requireNoError(t, owner.Start(context.Background()))
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	callOK(t, owner, map[string]any{
		"op": "createCollection", "collection": "messages", "kind": "document",
	})

	const batchSize = 2048
	batch := make([]map[string]any, batchSize)
	for index := range batch {
		batch[index] = map[string]any{"marker": "cancelled-batch", "index": index}
	}
	cancelContext, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	response, err := owner.Call(cancelContext, map[string]any{
		"op": "batchPutData", "collection": "messages", "batch": batch,
	})
	if err != nil {
		var operation *fylo.OperationError
		if !errors.As(err, &operation) {
			t.Fatalf("cancelled mutation = %v, want a bounded operation error", err)
		}
		if operation.Code == "EAMBIGUOUS" && !operation.Ambiguous {
			t.Fatalf("post-admission cancellation was not marked ambiguous: %v", err)
		}
		if operation.Code == "ETIMEOUT" && operation.Ambiguous {
			t.Fatalf("pre-admission cancellation was marked ambiguous: %v", err)
		}
		if operation.Code != "EAMBIGUOUS" && operation.Code != "ETIMEOUT" {
			t.Fatalf("cancelled mutation = %v, want ETIMEOUT or EAMBIGUOUS", err)
		}
	} else if !response.OK {
		t.Fatalf("batch cancellation machine response = %+v", response.Error)
	}
	found := callOK(t, owner, map[string]any{
		"op": "findDocs", "collection": "messages",
		"query": map[string]any{"$ops": []any{map[string]any{
			"marker": map[string]any{"$eq": "cancelled-batch"},
		}}},
	})
	var documents map[string]any
	requireNoError(t, json.Unmarshal(found.Result, &documents))
	if len(documents) != 0 && len(documents) != batchSize {
		t.Fatalf("cancelled transactional batch persisted %d/%d documents", len(documents), batchSize)
	}

	runtime.GC()
	before := runtime.NumGoroutine()
	const workers = 8
	const iterations = 20
	var group sync.WaitGroup
	failures := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for index := 0; index < iterations; index++ {
				put, err := owner.Call(context.Background(), map[string]any{
					"op": "putData", "collection": "messages",
					"data": map[string]any{"worker": worker, "index": index},
				})
				if err != nil || !put.OK {
					failures <- fmt.Errorf("put worker %d/%d: %v %+v", worker, index, err, put.Error)
					return
				}
				var id string
				if err := json.Unmarshal(put.Result, &id); err != nil {
					failures <- err
					return
				}
				get, err := owner.Call(context.Background(), map[string]any{
					"op": "getDoc", "collection": "messages", "id": id,
				})
				if err != nil || !get.OK {
					failures <- fmt.Errorf("get worker %d/%d: %v %+v", worker, index, err, get.Error)
					return
				}
			}
		}(worker)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+8 {
		t.Fatalf("mixed load leaked goroutines: before=%d after=%d", before, after)
	}
	stats := owner.Stats()
	if !stats.Ready || stats.Starts > 2 || stats.StderrBytes > 64<<10 {
		t.Fatalf("unbounded mixed-load supervisor state: %+v", stats)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func callOK(t *testing.T, owner *fylo.Supervisor, request map[string]any) fylo.Response {
	t.Helper()
	response, err := owner.Call(context.Background(), request)
	requireNoError(t, err)
	if !response.OK {
		t.Fatalf("FYLO %v failed: %+v", request["op"], response.Error)
	}
	return response
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// lockedFYLOVersion is the release runtime.lock.json pins. Phase 2 qualifies
// the engine that ships, so it moves with the lock.
const lockedFYLOVersion = "26.33.02"

func releasedFYLO(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("FYLO_TEST_BINARY")
	if binary == "" {
		t.Skip("FYLO_TEST_BINARY is required for released-runtime qualification")
	}
	absolute, err := filepath.Abs(binary)
	requireNoError(t, err)
	return absolute
}

type bridgeResponse struct {
	OK    bool `json:"ok"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error,omitempty"`
}

func bridgeRequest(t *testing.T, address, secret string, payload []byte, nonce string) bridgeResponse {
	t.Helper()
	connection, err := net.DialTimeout("tcp", address, 2*time.Second)
	requireNoError(t, err)
	defer connection.Close()
	timestamp := time.Now().UnixMilli()
	encoded := base64.RawStdEncoding.EncodeToString(payload)
	message := strconv.FormatInt(timestamp, 10) + "." + nonce + "." + encoded
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	frame := map[string]any{
		"version":   1,
		"timestamp": timestamp,
		"nonce":     nonce,
		"payload":   encoded,
		"signature": hex.EncodeToString(mac.Sum(nil)),
	}
	line, err := json.Marshal(frame)
	requireNoError(t, err)
	_, err = fmt.Fprintf(connection, "%s\n", line)
	requireNoError(t, err)
	reply, err := bufio.NewReader(connection).ReadBytes('\n')
	requireNoError(t, err)
	var response bridgeResponse
	requireNoError(t, json.Unmarshal(reply, &response))
	return response
}
