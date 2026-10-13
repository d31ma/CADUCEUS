// Package fylobridge exposes a narrow authenticated loopback transport to the
// legacy runtime while Go remains the only FYLO process owner.
package fylobridge

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/d31ma/caduceus/internal/platform/fylo"
)

const (
	protocolVersion       = 1
	maxClockSkew          = 30 * time.Second
	ComposedMaxFrameBytes = 64 << 20
)

var validNonce = regexp.MustCompile(`^[A-Za-z0-9_-]{20,128}$`)

// Caller is implemented by the persistent FYLO supervisor.
type Caller interface {
	Call(context.Context, map[string]any) (fylo.Response, error)
}

// Authorizer is implemented by the auth boundary. It answers the composed
// `auth.*` operations, which are the only way the legacy runtime reaches an
// authentication or authorization decision.
type Authorizer interface {
	Dispatch(context.Context, string, json.RawMessage) (any, error)
}

// Dispatcher answers one composed feature operation without exposing its
// underlying FYLO collections to the legacy runtime.
type Dispatcher interface {
	Dispatch(context.Context, string, json.RawMessage) (any, error)
}

// Options defines the private bridge boundary.
type Options struct {
	Address              string
	Secret               string
	MaxFrameBytes        int
	RequestTimeout       time.Duration
	MaxAttachmentBytes   int64
	AttachmentQuotaBytes int64
	IncomingRoot         string
	DataRoot             string
	AllowedOperations    []string
	AllowedCollections   []string
}

// Bridge validates and forwards requests to one FYLO owner and one auth
// boundary.
type Bridge struct {
	options     Options
	caller      Caller
	authorizer  Authorizer
	authOps     map[string]struct{}
	accounts    Dispatcher
	accountOps  map[string]struct{}
	operations  map[string]struct{}
	collections map[string]struct{}

	replayMu  sync.Mutex
	nonces    map[string]time.Time
	storageMu sync.Mutex
}

type frame struct {
	Version   int    `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

var defaultOperations = []string{
	"handshake", "backupStatus", "createCollection", "inspectCollection",
	"rebuildCollection", "verifyCollection", "getDoc", "getFileData", "getLatest", "getMeta",
	"setMeta", "findDocs", "findDeletedDocs", "restoreDoc", "joinDocs",
	"putData", "batchPutData", "patchDoc", "patchDocs", "delDoc", "delDocs",
}

var defaultCollections = fylo.AllCollections()

// New returns a fail-closed bridge. Listen performs configuration validation.
func New(options Options, caller Caller) *Bridge {
	return &Bridge{
		options:     options,
		caller:      caller,
		operations:  stringSet(options.AllowedOperations, defaultOperations),
		collections: stringSet(options.AllowedCollections, defaultCollections),
		nonces:      make(map[string]time.Time),
	}
}

// WithAuthorizer attaches the auth boundary and its operation allowlist. A
// bridge without one refuses every `auth.*` operation rather than falling
// through to FYLO, so a misconfigured runtime cannot answer a login by
// accident.
func (bridge *Bridge) WithAuthorizer(authorizer Authorizer, operations []string) *Bridge {
	bridge.authorizer = authorizer
	bridge.authOps = stringSet(operations, nil)
	return bridge
}

// WithAccounts attaches the typed external-account boundary. Its operations
// have a separate allowlist and cannot fall through to raw FYLO forwarding.
func (bridge *Bridge) WithAccounts(dispatcher Dispatcher, operations []string) *Bridge {
	bridge.accounts = dispatcher
	bridge.accountOps = stringSet(operations, nil)
	return bridge
}

// Listen binds the configured loopback-only TCP address.
func (bridge *Bridge) Listen() (net.Listener, error) {
	if bridge.caller == nil {
		return nil, errors.New("FYLO bridge caller is required")
	}
	if len([]byte(bridge.options.Secret)) < 32 {
		return nil, errors.New("FYLO bridge secret must contain at least 32 bytes")
	}
	if bridge.options.Address == "" {
		bridge.options.Address = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(bridge.options.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid FYLO bridge address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("FYLO bridge must bind to an explicit loopback IP")
	}
	if bridge.options.MaxFrameBytes < 1024 || bridge.options.MaxFrameBytes > 64<<20 {
		return nil, errors.New("FYLO bridge frame limit must be from 1 KiB through 64 MiB")
	}
	if bridge.options.RequestTimeout < 100*time.Millisecond ||
		bridge.options.RequestTimeout > 2*time.Minute {
		return nil, errors.New("FYLO bridge timeout must be from 100ms through 2m")
	}
	incoming, err := filepath.Abs(bridge.options.IncomingRoot)
	if err != nil || incoming == "" {
		return nil, errors.New("FYLO bridge incoming root must be an absolute path")
	}
	bridge.options.IncomingRoot = filepath.Clean(incoming)
	if bridge.options.DataRoot == "" {
		bridge.options.DataRoot = filepath.Dir(bridge.options.IncomingRoot)
	}
	dataRoot, err := filepath.Abs(bridge.options.DataRoot)
	if err != nil {
		return nil, errors.New("FYLO bridge data root must be an absolute path")
	}
	bridge.options.DataRoot = filepath.Clean(dataRoot)
	if err := os.MkdirAll(bridge.options.IncomingRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create FYLO incoming root: %w", err)
	}
	return net.Listen("tcp", bridge.options.Address)
}

// Serve accepts bounded one-request connections until cancellation.
func (bridge *Bridge) Serve(ctx context.Context, listener net.Listener) error {
	if listener == nil {
		return errors.New("FYLO bridge listener is required")
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept FYLO bridge connection: %w", err)
		}
		go bridge.serveConnection(ctx, connection)
	}
}

func (bridge *Bridge) serveConnection(parent context.Context, connection net.Conn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(bridge.options.RequestTimeout))
	reader := bufio.NewReaderSize(connection, bridge.options.MaxFrameBytes+1)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		bridge.writeError(connection, "EFRAME", "read one complete bridge frame")
		return
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	if len(line) > bridge.options.MaxFrameBytes {
		bridge.writeError(connection, "EFRAME_REQUEST_TOO_LARGE", "bridge frame exceeds configured limit")
		return
	}
	request, code, err := bridge.authenticate(line)
	if err != nil {
		bridge.writeError(connection, code, err.Error())
		return
	}
	if operation, ok := request["op"].(string); ok && strings.HasPrefix(operation, authOperationPrefix) {
		bridge.serveAuth(parent, connection, operation, request)
		return
	}
	if operation, ok := request["op"].(string); ok && strings.HasPrefix(operation, accountsOperationPrefix) {
		bridge.serveAccounts(parent, connection, operation, request)
		return
	}
	fileMutation := request["op"] == "putData" && request["file"] != nil
	if fileMutation {
		bridge.storageMu.Lock()
		defer bridge.storageMu.Unlock()
	}
	if code, err := bridge.authorize(request); err != nil {
		bridge.writeError(connection, code, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(parent, bridge.options.RequestTimeout)
	defer cancel()
	response, err := bridge.caller.Call(ctx, request)
	if err != nil {
		code := "EUNAVAILABLE"
		var operation *fylo.OperationError
		if errors.As(err, &operation) {
			code = operation.Code
		}
		bridge.writeError(connection, code, err.Error())
		return
	}
	_, _ = connection.Write(append(response.Bytes(), '\n'))
}

// authOperationPrefix marks the composed authentication and authorization
// operations. It is duplicated from the auth package rather than imported so
// this transport keeps no dependency on the boundary it forwards to.
const authOperationPrefix = "auth."
const accountsOperationPrefix = "accounts."

// serveAuth forwards one composed auth operation. The FYLO allowlists do not
// apply: an auth frame carries no collection and no file, and its own
// allowlist is the one the boundary published.
func (bridge *Bridge) serveAuth(
	parent context.Context,
	connection net.Conn,
	operation string,
	request map[string]any,
) {
	_, allowed := bridge.authOps[operation]
	bridge.serveComposed(parent, connection, operation, request, bridge.authorizer, allowed, "auth")
}

func (bridge *Bridge) serveAccounts(
	parent context.Context,
	connection net.Conn,
	operation string,
	request map[string]any,
) {
	_, allowed := bridge.accountOps[operation]
	bridge.serveComposed(parent, connection, operation, request, bridge.accounts, allowed, "accounts")
}

func (bridge *Bridge) serveComposed(
	parent context.Context,
	connection net.Conn,
	operation string,
	request map[string]any,
	dispatcher Dispatcher,
	allowed bool,
	label string,
) {
	if dispatcher == nil {
		bridge.writeError(connection, "EFORBIDDEN", "this bridge serves no "+label+" boundary")
		return
	}
	if !allowed {
		bridge.writeError(connection, "EFORBIDDEN",
			fmt.Sprintf("%s operation %q is not bridge-allowlisted", label, operation))
		return
	}
	payload, err := json.Marshal(request["params"])
	if err != nil {
		bridge.writeError(connection, "EBADREQUEST", "auth parameters are not encodable")
		return
	}
	if request["params"] == nil {
		payload = []byte("{}")
	}
	ctx, cancel := context.WithTimeout(parent, bridge.options.RequestTimeout)
	defer cancel()
	result, err := dispatcher.Dispatch(ctx, operation, payload)
	if err != nil {
		code, message := authFailure(err)
		bridge.writeError(connection, code, message)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		bridge.writeError(connection, "EUNAVAILABLE", "auth result is not encodable")
		return
	}
	response := fylo.Response{
		ProtocolVersion: protocolVersion,
		OK:              true,
		Operation:       operation,
		Result:          encoded,
	}
	body, err := json.Marshal(response)
	if err != nil {
		bridge.writeError(connection, "EUNAVAILABLE", "auth response is not encodable")
		return
	}
	_, _ = connection.Write(append(body, '\n'))
}

// authFailure extracts the stable code the boundary assigned. The boundary's
// Failure type is matched structurally so this package stays independent of it.
func authFailure(err error) (string, string) {
	var coded interface{ Error() string }
	coded = err
	message := coded.Error()
	if index := strings.Index(message, ": "); index > 0 {
		code := message[:index]
		if isStableCode(code) {
			return code, message[index+2:]
		}
	}
	return "EUNAVAILABLE", message
}

func isStableCode(code string) bool {
	if len(code) < 2 || code[0] != 'E' {
		return false
	}
	for _, character := range code[1:] {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func (bridge *Bridge) authenticate(line []byte) (map[string]any, string, error) {
	var envelope frame
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, "EFRAME", fmt.Errorf("invalid bridge envelope: %w", err)
	}
	if envelope.Version != protocolVersion {
		return nil, "EPROTOCOL", errors.New("unsupported bridge protocol")
	}
	now := time.Now()
	stamp := time.UnixMilli(envelope.Timestamp)
	if stamp.Before(now.Add(-maxClockSkew)) || stamp.After(now.Add(maxClockSkew)) {
		return nil, "ESTALE", errors.New("bridge timestamp is outside the permitted window")
	}
	if !validNonce.MatchString(envelope.Nonce) {
		return nil, "EAUTH", errors.New("bridge nonce is invalid")
	}
	signature, err := hex.DecodeString(envelope.Signature)
	if err != nil || len(signature) != sha256.Size {
		return nil, "EAUTH", errors.New("bridge signature is invalid")
	}
	message := fmt.Sprintf("%d.%s.%s", envelope.Timestamp, envelope.Nonce, envelope.Payload)
	mac := hmac.New(sha256.New, []byte(bridge.options.Secret))
	_, _ = io.WriteString(mac, message)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, "EAUTH", errors.New("bridge signature does not match")
	}
	if !bridge.rememberNonce(envelope.Nonce, now) {
		return nil, "EREPLAY", errors.New("bridge nonce has already been used")
	}
	payload, err := decodePayload(envelope.Payload)
	if err != nil || len(payload) > bridge.options.MaxFrameBytes {
		return nil, "EFRAME", errors.New("bridge payload is invalid")
	}
	var request map[string]any
	decoder = json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return nil, "EBADREQUEST", fmt.Errorf("invalid FYLO payload: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, "EBADREQUEST", errors.New("FYLO payload must contain one JSON object")
	}
	return request, "", nil
}

func decodePayload(encoded string) ([]byte, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err == nil {
		return payload, nil
	}
	return base64.RawStdEncoding.DecodeString(encoded)
}

func (bridge *Bridge) rememberNonce(nonce string, now time.Time) bool {
	bridge.replayMu.Lock()
	defer bridge.replayMu.Unlock()
	for candidate, expires := range bridge.nonces {
		if !expires.After(now) {
			delete(bridge.nonces, candidate)
		}
	}
	if _, found := bridge.nonces[nonce]; found {
		return false
	}
	bridge.nonces[nonce] = now.Add(maxClockSkew)
	return true
}

func (bridge *Bridge) authorize(request map[string]any) (string, error) {
	operation, ok := request["op"].(string)
	if !ok {
		return "EBADREQUEST", errors.New("FYLO operation is required")
	}
	if _, allowed := bridge.operations[operation]; !allowed {
		return "EFORBIDDEN", fmt.Errorf("FYLO operation %q is not bridge-allowlisted", operation)
	}
	for _, forbidden := range []string{"root", "schemaDir", "worm", "versioning", "url"} {
		if _, found := request[forbidden]; found {
			return "EFORBIDDEN", fmt.Errorf("caller-controlled %s is forbidden", forbidden)
		}
	}
	if collection, ok := request["collection"].(string); ok && len(bridge.collections) > 0 {
		if _, allowed := bridge.collections[collection]; !allowed {
			return "EFORBIDDEN", fmt.Errorf("FYLO collection %q is not bridge-allowlisted", collection)
		}
	}
	if operation == "putData" {
		if code, err := bridge.authorizeFile(request); err != nil {
			return code, err
		}
	}
	if operation == "getFileData" {
		if _, pathRequested := request["path"]; pathRequested {
			return "EFORBIDDEN", errors.New("bridge bucket reads must use bounded inline output")
		}
	}
	return "", nil
}

func (bridge *Bridge) authorizeFile(request map[string]any) (string, error) {
	file, found := request["file"].(map[string]any)
	if !found {
		return "", nil
	}
	raw, ok := file["path"].(string)
	if !ok {
		return "EFORBIDDEN", errors.New("bridge file ingestion requires an incoming path")
	}
	resolved, err := filepath.Abs(raw)
	if err != nil {
		return "EFORBIDDEN", errors.New("invalid incoming file path")
	}
	relative, err := filepath.Rel(bridge.options.IncomingRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "EFORBIDDEN", errors.New("incoming file is outside the bridge root")
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "EFORBIDDEN", errors.New("incoming file must be a regular non-symlink file")
	}
	if bridge.options.MaxAttachmentBytes > 0 && info.Size() > bridge.options.MaxAttachmentBytes {
		return "EATTACHMENTLIMIT", errors.New("incoming attachment exceeds its byte limit")
	}
	if bridge.options.MaxAttachmentBytes > 0 {
		options, _ := request["fileOptions"].(map[string]any)
		if options == nil {
			options = map[string]any{}
			request["fileOptions"] = options
		}
		options["maxBytes"] = bridge.options.MaxAttachmentBytes
	}
	if bridge.options.AttachmentQuotaBytes > 0 {
		used, err := bucketBytes(filepath.Join(
			bridge.options.DataRoot, ".buckets", "attachment-files", "docs",
		))
		if err != nil {
			return "EQUOTA", fmt.Errorf("measure attachment quota: %w", err)
		}
		if used > bridge.options.AttachmentQuotaBytes-info.Size() {
			return "EQUOTA", errors.New("attachment bucket quota would be exceeded")
		}
	}
	return "", nil
}

func bucketBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is forbidden in attachment bucket: %s", path)
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return total, err
}

func (bridge *Bridge) writeError(writer io.Writer, code, message string) {
	response := fylo.Response{
		ProtocolVersion: protocolVersion,
		OK:              false,
		Error: &fylo.MachineError{
			Name:    "CaduceusBridgeError",
			Code:    code,
			Message: message,
		},
	}
	encoded, _ := json.Marshal(response)
	_, _ = writer.Write(append(encoded, '\n'))
}

func stringSet(values, fallback []string) map[string]struct{} {
	if len(values) == 0 {
		values = fallback
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}
