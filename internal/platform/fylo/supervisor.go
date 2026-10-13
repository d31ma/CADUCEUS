package fylo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

var releaseCommit = regexp.MustCompile(`^[a-f0-9]{40}$`)

type childProcess struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	writer  *bufio.Writer
	reader  *bufio.Reader
	done    chan error
	logs    *boundedBuffer
}

// Supervisor owns one persistent, exclusive-root FYLO process.
type Supervisor struct {
	options Options

	calls    chan struct{}
	state    sync.RWMutex
	child    *childProcess
	identity Identity
	ready    atomic.Bool
	closing  atomic.Bool

	starts    atomic.Uint64
	restarts  atomic.Uint64
	requests  atomic.Uint64
	failures  atomic.Uint64
	timeouts  atomic.Uint64
	ambiguous atomic.Uint64
	requestID atomic.Uint64
}

// New constructs a supervisor. Configuration errors are reported by Start.
func New(options Options) *Supervisor {
	return &Supervisor{options: options, calls: make(chan struct{}, 1)}
}

// Start launches and validates the exclusive FYLO owner.
func (supervisor *Supervisor) Start(ctx context.Context) error {
	if err := supervisor.acquire(ctx); err != nil {
		return err
	}
	defer supervisor.release()
	if supervisor.ready.Load() {
		return nil
	}
	options, err := normalizedOptions(supervisor.options)
	if err != nil {
		return err
	}
	supervisor.options = options
	supervisor.closing.Store(false)
	return supervisor.startLocked(ctx, false)
}

func (supervisor *Supervisor) startLocked(ctx context.Context, restart bool) error {
	if restart && int(supervisor.restarts.Load()) >= supervisor.options.RestartLimit {
		return &OperationError{
			Code:      "ERESTARTLIMIT",
			Operation: "handshake",
			Cause:     errors.New("FYLO restart budget exhausted"),
		}
	}
	arguments := []string{
		"exec",
		"--loop",
		"--root",
		supervisor.options.Root,
		"--exclusive-root",
		"--max-request-bytes",
		strconv.Itoa(supervisor.options.MaxRequestBytes),
		"--max-response-bytes",
		strconv.Itoa(supervisor.options.MaxResponseBytes),
	}
	command := exec.Command(supervisor.options.Binary, arguments...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("open FYLO stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("open FYLO stdout: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("open FYLO stderr: %w", err)
	}
	process := &childProcess{
		command: command,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		writer:  bufio.NewWriterSize(stdin, supervisor.options.MaxRequestBytes+1),
		reader:  bufio.NewReaderSize(stdout, supervisor.options.MaxResponseBytes+1),
		done:    make(chan error, 1),
		logs:    newBoundedBuffer(64 << 10),
	}
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start FYLO: %w", err)
	}
	go func() {
		_, _ = io.Copy(process.logs, stderr)
	}()
	supervisor.setChild(process)
	go supervisor.waitForExit(process)
	supervisor.starts.Add(1)
	if restart {
		supervisor.restarts.Add(1)
	}

	handshakeContext, cancel := supervisor.withTimeout(ctx)
	defer cancel()
	response, err := supervisor.roundTrip(handshakeContext, process, map[string]any{
		"op": "handshake",
	})
	if err != nil {
		supervisor.failChild(process)
		return err
	}
	if !response.OK {
		supervisor.failChild(process)
		if response.Error != nil {
			return response.Error
		}
		return errors.New("FYLO handshake failed without an error")
	}
	var identity Identity
	if err := json.Unmarshal(response.Result, &identity); err != nil {
		supervisor.failChild(process)
		return fmt.Errorf("decode FYLO handshake: %w", err)
	}
	if err := supervisor.validateIdentity(identity); err != nil {
		supervisor.failChild(process)
		return err
	}
	supervisor.state.Lock()
	if supervisor.child == process {
		supervisor.identity = identity
		supervisor.ready.Store(true)
	}
	supervisor.state.Unlock()
	return nil
}

// Call sends one bounded machine request. Machine-level failures remain in the
// returned Response; transport and framing failures are Go errors.
func (supervisor *Supervisor) Call(
	ctx context.Context,
	request map[string]any,
) (Response, error) {
	operation, _ := request["op"].(string)
	if operation == "" {
		return Response{}, errors.New("FYLO request requires an operation")
	}
	callContext, cancel := supervisor.withTimeout(ctx)
	defer cancel()
	if err := supervisor.acquire(callContext); err != nil {
		supervisor.timeouts.Add(1)
		return Response{}, &OperationError{
			Code:      "ETIMEOUT",
			Operation: operation,
			Cause:     err,
		}
	}
	defer supervisor.release()
	if supervisor.closing.Load() {
		return Response{}, errors.New("FYLO supervisor is closing")
	}
	if !supervisor.ready.Load() {
		if err := supervisor.startLocked(callContext, supervisor.starts.Load() > 0); err != nil {
			return Response{}, err
		}
	}
	process := supervisor.currentChild()
	supervisor.requests.Add(1)
	response, err := supervisor.roundTrip(callContext, process, request)
	if err == nil {
		return response, nil
	}
	supervisor.failures.Add(1)
	supervisor.failChild(process)
	cleanupContext, cleanupCancel := context.WithTimeout(
		context.Background(), supervisor.options.ShutdownTimeout,
	)
	_ = supervisor.stopChild(cleanupContext, process, true)
	cleanupCancel()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		supervisor.timeouts.Add(1)
		ambiguous := IsMutation(operation)
		if ambiguous {
			supervisor.ambiguous.Add(1)
		}
		return Response{}, &OperationError{
			Code:      map[bool]string{true: "EAMBIGUOUS", false: "ETIMEOUT"}[ambiguous],
			Operation: operation,
			Ambiguous: ambiguous,
			Cause:     err,
		}
	}
	return Response{}, &OperationError{
		Code:      "EUNAVAILABLE",
		Operation: operation,
		Ambiguous: IsMutation(operation),
		Cause:     err,
	}
}

// Restart replaces the current child and revalidates its identity and lease.
func (supervisor *Supervisor) Restart(ctx context.Context) error {
	if err := supervisor.acquire(ctx); err != nil {
		return err
	}
	defer supervisor.release()
	if supervisor.closing.Load() {
		return errors.New("FYLO supervisor is closing")
	}
	if child := supervisor.currentChild(); child != nil {
		if err := supervisor.stopChild(ctx, child, true); err != nil {
			return err
		}
	}
	return supervisor.startLocked(ctx, supervisor.starts.Load() > 0)
}

// WithQuiescedRoot runs filesystem work while the validated child retains the
// exclusive root lease and all FYLO calls are blocked.
func (supervisor *Supervisor) WithQuiescedRoot(
	ctx context.Context,
	work func(root string) error,
) error {
	if err := supervisor.acquire(ctx); err != nil {
		return err
	}
	defer supervisor.release()
	if supervisor.closing.Load() {
		return errors.New("FYLO supervisor is closing")
	}
	if !supervisor.ready.Load() {
		if err := supervisor.startLocked(ctx, supervisor.starts.Load() > 0); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return work(supervisor.options.Root)
}

// Close releases the root lease within the configured shutdown bound.
func (supervisor *Supervisor) Close(ctx context.Context) error {
	if err := supervisor.acquire(ctx); err != nil {
		return err
	}
	defer supervisor.release()
	if supervisor.closing.Swap(true) {
		return nil
	}
	process := supervisor.currentChild()
	if process == nil {
		supervisor.ready.Store(false)
		return nil
	}
	shutdownContext, cancel := context.WithTimeout(ctx, supervisor.options.ShutdownTimeout)
	defer cancel()
	return supervisor.stopChild(shutdownContext, process, false)
}

// Ready reports whether a validated child currently owns the root.
func (supervisor *Supervisor) Ready() bool {
	return supervisor.ready.Load()
}

// Identity returns the most recently validated handshake.
func (supervisor *Supervisor) Identity() Identity {
	supervisor.state.RLock()
	defer supervisor.state.RUnlock()
	return supervisor.identity
}

// Stats returns bounded process measurements.
func (supervisor *Supervisor) Stats() Stats {
	var stderrBytes uint64
	if process := supervisor.currentChild(); process != nil {
		stderrBytes = uint64(process.logs.Len())
	}
	return Stats{
		Starts:      supervisor.starts.Load(),
		Restarts:    supervisor.restarts.Load(),
		Requests:    supervisor.requests.Load(),
		Failures:    supervisor.failures.Load(),
		Timeouts:    supervisor.timeouts.Load(),
		Ambiguous:   supervisor.ambiguous.Load(),
		Ready:       supervisor.ready.Load(),
		StderrBytes: stderrBytes,
	}
}

func (supervisor *Supervisor) roundTrip(
	ctx context.Context,
	process *childProcess,
	request map[string]any,
) (Response, error) {
	safeRequest := make(map[string]any, len(request)+1)
	for name, value := range request {
		safeRequest[name] = value
	}
	if _, exists := safeRequest["requestId"]; !exists {
		safeRequest["requestId"] = fmt.Sprintf("caduceus-%d", supervisor.requestID.Add(1))
	}
	line, err := json.Marshal(safeRequest)
	if err != nil {
		return Response{}, fmt.Errorf("encode FYLO request: %w", err)
	}
	if len(line) > supervisor.options.MaxRequestBytes {
		return Response{}, &MachineError{
			Code:    "EFRAME_REQUEST_TOO_LARGE",
			Message: fmt.Sprintf("FYLO request exceeds %d bytes", supervisor.options.MaxRequestBytes),
		}
	}
	type result struct {
		response Response
		err      error
	}
	completed := make(chan result, 1)
	go func() {
		response, callErr := exchange(process, line, supervisor.options.MaxResponseBytes)
		completed <- result{response: response, err: callErr}
	}()
	select {
	case outcome := <-completed:
		return outcome.response, outcome.err
	case <-ctx.Done():
		terminateChild(process)
		return Response{}, ctx.Err()
	}
}

func exchange(process *childProcess, request []byte, maxResponseBytes int) (Response, error) {
	if _, err := process.writer.Write(request); err != nil {
		return Response{}, fmt.Errorf("write FYLO request: %w", err)
	}
	if err := process.writer.WriteByte('\n'); err != nil {
		return Response{}, fmt.Errorf("write FYLO delimiter: %w", err)
	}
	if err := process.writer.Flush(); err != nil {
		return Response{}, fmt.Errorf("flush FYLO request: %w", err)
	}
	reply, err := process.reader.ReadSlice('\n')
	if err != nil {
		return Response{}, fmt.Errorf("read FYLO response: %w", err)
	}
	reply = bytes.TrimSuffix(reply, []byte{'\n'})
	if len(reply) > maxResponseBytes {
		return Response{}, fmt.Errorf("FYLO response exceeds %d bytes", maxResponseBytes)
	}
	if !utf8.Valid(reply) {
		return Response{}, errors.New("FYLO response is not valid UTF-8")
	}
	var response Response
	decoder := json.NewDecoder(bytes.NewReader(reply))
	if err := decoder.Decode(&response); err != nil {
		return Response{}, fmt.Errorf("decode FYLO response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Response{}, errors.New("FYLO response contains trailing JSON")
	}
	if response.ProtocolVersion != 1 {
		return Response{}, fmt.Errorf("unexpected FYLO response protocol %d", response.ProtocolVersion)
	}
	if !response.OK && response.Error == nil {
		return Response{}, errors.New("FYLO failure response has no stable error")
	}
	response.raw = append([]byte(nil), reply...)
	return response, nil
}

func (supervisor *Supervisor) validateIdentity(identity Identity) error {
	if identity.RuntimeVersion != supervisor.options.ExpectedVersion {
		return fmt.Errorf(
			"FYLO runtime version %q does not match %q",
			identity.RuntimeVersion,
			supervisor.options.ExpectedVersion,
		)
	}
	if identity.ProtocolVersion != 1 {
		return fmt.Errorf("FYLO protocol version %d is unsupported", identity.ProtocolVersion)
	}
	if identity.BuildKind != "release" || !releaseCommit.MatchString(identity.Commit) {
		return errors.New("FYLO must identify as an immutable release build")
	}
	if supervisor.options.ExpectedCommit != "" && identity.Commit != supervisor.options.ExpectedCommit {
		return errors.New("FYLO release commit differs from runtime lock")
	}
	if supervisor.options.ExpectedTarget != "" &&
		identity.BuildTarget != supervisor.options.ExpectedTarget {
		return fmt.Errorf(
			"FYLO build target %q does not match %q",
			identity.BuildTarget,
			supervisor.options.ExpectedTarget,
		)
	}
	if identity.Machine.Framing != "ndjson" ||
		identity.Machine.Encoding != "utf-8" ||
		identity.Machine.Delimiter != "LF" ||
		identity.Machine.DuplicateKeys != "rejected" {
		return errors.New("FYLO framing contract is unsupported")
	}
	if identity.Machine.MaxRequestBytes != supervisor.options.MaxRequestBytes ||
		identity.Machine.MaxResponseBytes != supervisor.options.MaxResponseBytes {
		return errors.New("FYLO framing limits differ from supervisor configuration")
	}
	for _, dependency := range []string{"chex", "ttid"} {
		reported, found := identity.Dependencies[dependency]
		if !found || reported.RequiredVersion == "" || !reported.Available {
			return fmt.Errorf("FYLO dependency %s is unavailable", dependency)
		}
		if expected := supervisor.options.ExpectedDependencies[dependency]; expected != "" &&
			reported.RequiredVersion != expected {
			return fmt.Errorf(
				"FYLO dependency %s version %q does not match %q",
				dependency,
				reported.RequiredVersion,
				expected,
			)
		}
	}
	required := supervisor.options.RequiredCapabilities
	if len(required) == 0 {
		required = []string{"handshake", "exclusiveRoot", "queryPagination"}
	}
	for _, capability := range required {
		value, found := identity.Capabilities[capability]
		if !found || value == false || value == nil {
			return fmt.Errorf("FYLO capability %s is unavailable", capability)
		}
	}
	return nil
}

func (supervisor *Supervisor) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := supervisor.options.RequestTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (supervisor *Supervisor) acquire(ctx context.Context) error {
	select {
	case supervisor.calls <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (supervisor *Supervisor) release() {
	<-supervisor.calls
}

func (supervisor *Supervisor) setChild(process *childProcess) {
	supervisor.state.Lock()
	supervisor.child = process
	supervisor.ready.Store(false)
	supervisor.state.Unlock()
}

func (supervisor *Supervisor) currentChild() *childProcess {
	supervisor.state.RLock()
	defer supervisor.state.RUnlock()
	return supervisor.child
}

func (supervisor *Supervisor) waitForExit(process *childProcess) {
	err := process.command.Wait()
	process.done <- err
	close(process.done)
	supervisor.state.Lock()
	if supervisor.child == process {
		supervisor.ready.Store(false)
	}
	supervisor.state.Unlock()
}

func (supervisor *Supervisor) failChild(process *childProcess) {
	supervisor.ready.Store(false)
	terminateChild(process)
}

func (supervisor *Supervisor) stopChild(
	ctx context.Context,
	process *childProcess,
	force bool,
) error {
	supervisor.ready.Store(false)
	if force {
		terminateChild(process)
	} else {
		_ = process.stdin.Close()
	}
	select {
	case err := <-process.done:
		supervisor.clearChild(process)
		if err != nil && !force {
			return fmt.Errorf("stop FYLO: %w", err)
		}
		return nil
	case <-ctx.Done():
		terminateChild(process)
		supervisor.clearChild(process)
		return ctx.Err()
	}
}

func terminateChild(process *childProcess) {
	if process == nil {
		return
	}
	if process.command != nil && process.command.Process != nil {
		_ = process.command.Process.Kill()
	}
	if process.stdout != nil {
		_ = process.stdout.Close()
	}
	if process.stderr != nil {
		_ = process.stderr.Close()
	}
}

func (supervisor *Supervisor) clearChild(process *childProcess) {
	supervisor.state.Lock()
	if supervisor.child == process {
		supervisor.child = nil
	}
	supervisor.state.Unlock()
}

type boundedBuffer struct {
	mu       sync.Mutex
	capacity int
	data     []byte
}

func newBoundedBuffer(capacity int) *boundedBuffer {
	return &boundedBuffer{capacity: capacity}
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	original := len(data)
	if len(data) >= buffer.capacity {
		buffer.data = append(buffer.data[:0], data[len(data)-buffer.capacity:]...)
		return original, nil
	}
	overflow := len(buffer.data) + len(data) - buffer.capacity
	if overflow > 0 {
		copy(buffer.data, buffer.data[overflow:])
		buffer.data = buffer.data[:len(buffer.data)-overflow]
	}
	buffer.data = append(buffer.data, data...)
	return original, nil
}

func (buffer *boundedBuffer) Len() int {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return len(buffer.data)
}
