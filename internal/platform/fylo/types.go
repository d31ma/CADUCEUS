// Package fylo supervises the released FYLO machine-protocol process.
package fylo

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultMaxRequestBytes  = 1 << 20
	DefaultMaxResponseBytes = 8 << 20
)

// Options defines the immutable process and framing contract.
type Options struct {
	Binary          string
	Root            string
	ExpectedVersion string
	// ExpectedDependencies pins the CHEX and TTID versions reported by the
	// FYLO handshake to the same release lock used for installation.
	ExpectedDependencies map[string]string
	// RequiredCapabilities is the handshake capability set this deployment
	// depends on, taken from the runtime lock. An empty list falls back to the
	// three every CADUCEUS feature needs.
	RequiredCapabilities []string
	ExpectedCommit       string
	ExpectedTarget       string
	MaxRequestBytes      int
	MaxResponseBytes     int
	RequestTimeout       time.Duration
	ShutdownTimeout      time.Duration
	RestartLimit         int
}

// Identity is the stable subset of the FYLO handshake consumed by CADUCEUS.
type Identity struct {
	RuntimeVersion  string                `json:"runtimeVersion"`
	ProtocolVersion int                   `json:"protocolVersion"`
	Commit          string                `json:"commit"`
	BuildTarget     string                `json:"buildTarget"`
	BuildKind       string                `json:"buildKind"`
	Dependencies    map[string]Dependency `json:"dependencies"`
	Machine         MachineContract       `json:"machine"`
	Capabilities    map[string]any        `json:"capabilities"`
}

// Dependency records one binary dependency reported by FYLO.
type Dependency struct {
	RequiredVersion string `json:"requiredVersion"`
	Available       bool   `json:"available"`
}

// MachineContract records the negotiated framing limits.
type MachineContract struct {
	Framing          string `json:"framing"`
	Encoding         string `json:"encoding"`
	Delimiter        string `json:"delimiter"`
	MaxRequestBytes  int    `json:"maxRequestBytes"`
	MaxResponseBytes int    `json:"maxResponseBytes"`
	DuplicateKeys    string `json:"duplicateKeys"`
}

// Response is one complete FYLO machine response.
type Response struct {
	ProtocolVersion int             `json:"protocolVersion"`
	OK              bool            `json:"ok"`
	Operation       string          `json:"op"`
	RequestID       any             `json:"requestId"`
	DurationMS      float64         `json:"durationMs"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *MachineError   `json:"error,omitempty"`

	raw []byte
}

// Bytes returns the exact validated machine response.
func (response Response) Bytes() []byte {
	if len(response.raw) != 0 {
		return append([]byte(nil), response.raw...)
	}
	encoded, _ := json.Marshal(response)
	return encoded
}

// MachineError is a stable error returned by FYLO or the private bridge.
type MachineError struct {
	Name    string `json:"name,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (failure *MachineError) Error() string {
	if failure == nil {
		return "FYLO machine error"
	}
	if failure.Code == "" {
		return failure.Message
	}
	return failure.Code + ": " + failure.Message
}

// OperationError represents a transport failure whose mutation outcome may be
// ambiguous.
type OperationError struct {
	Code      string
	Operation string
	Ambiguous bool
	Cause     error
}

func (failure *OperationError) Error() string {
	label := "FYLO " + failure.Operation
	if failure.Ambiguous {
		label += " outcome is ambiguous"
	}
	return fmt.Sprintf("%s: %v", label, failure.Cause)
}

func (failure *OperationError) Unwrap() error {
	return failure.Cause
}

// HasCode reports whether an error carries one stable FYLO/CADUCEUS code.
func HasCode(err error, code string) bool {
	var machine *MachineError
	if errors.As(err, &machine) {
		return machine.Code == code
	}
	var operation *OperationError
	return errors.As(err, &operation) && operation.Code == code
}

// Stats is a secret-free supervisor measurement snapshot.
type Stats struct {
	Starts      uint64 `json:"starts"`
	Restarts    uint64 `json:"restarts"`
	Requests    uint64 `json:"requests"`
	Failures    uint64 `json:"failures"`
	Timeouts    uint64 `json:"timeouts"`
	Ambiguous   uint64 `json:"ambiguous"`
	Ready       bool   `json:"ready"`
	StderrBytes uint64 `json:"stderrBytes"`
}

// QualificationOptions defines the released-runtime proving harness.
type QualificationOptions struct {
	Binary               string
	ExpectedVersion      string
	MaxRequestBytes      int
	MaxResponseBytes     int
	MaxAttachmentBytes   int64
	AttachmentQuotaBytes int64
}

// QualificationReport is serialized as the Phase 2 evidence contract.
type QualificationReport struct {
	SchemaVersion   int             `json:"schemaVersion"`
	RuntimeVersion  string          `json:"runtimeVersion"`
	ProtocolVersion int             `json:"protocolVersion"`
	ReleaseCommit   string          `json:"releaseCommit"`
	BuildTarget     string          `json:"buildTarget"`
	Checks          map[string]bool `json:"checks"`
	Limits          Limits          `json:"limits"`
}

// Limits records the enforced framing and attachment bounds.
type Limits struct {
	MaxRequestBytes      int   `json:"maxRequestBytes"`
	MaxResponseBytes     int   `json:"maxResponseBytes"`
	MaxAttachmentBytes   int64 `json:"maxAttachmentBytes"`
	AttachmentQuotaBytes int64 `json:"attachmentQuotaBytes"`
}

func normalizedOptions(options Options) (Options, error) {
	if strings.TrimSpace(options.Binary) == "" {
		return Options{}, errors.New("FYLO binary is required")
	}
	if strings.TrimSpace(options.Root) == "" {
		return Options{}, errors.New("FYLO root is required")
	}
	if strings.TrimSpace(options.ExpectedVersion) == "" {
		return Options{}, errors.New("expected FYLO version is required")
	}
	if options.MaxRequestBytes == 0 {
		options.MaxRequestBytes = DefaultMaxRequestBytes
	}
	if options.MaxResponseBytes == 0 {
		options.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if options.RequestTimeout == 0 {
		options.RequestTimeout = 10 * time.Second
	}
	if options.ShutdownTimeout == 0 {
		options.ShutdownTimeout = 10 * time.Second
	}
	if options.RestartLimit == 0 {
		options.RestartLimit = 3
	}
	if options.MaxRequestBytes < 256 || options.MaxRequestBytes > 64<<20 {
		return Options{}, errors.New("FYLO max request bytes must be from 256 bytes through 64 MiB")
	}
	if options.MaxResponseBytes < 1024 || options.MaxResponseBytes > 64<<20 {
		return Options{}, errors.New("FYLO max response bytes must be from 1 KiB through 64 MiB")
	}
	if options.RequestTimeout < 100*time.Millisecond || options.RequestTimeout > 2*time.Minute {
		return Options{}, errors.New("FYLO request timeout must be from 100ms through 2m")
	}
	if options.ShutdownTimeout < time.Second || options.ShutdownTimeout > 2*time.Minute {
		return Options{}, errors.New("FYLO shutdown timeout must be from 1s through 2m")
	}
	if options.RestartLimit < 0 || options.RestartLimit > 100 {
		return Options{}, errors.New("FYLO restart limit must be from 0 through 100")
	}
	return options, nil
}

func platformTarget() string {
	operatingSystem := runtime.GOOS
	if operatingSystem == "darwin" {
		operatingSystem = "macos"
	}
	return operatingSystem + "-" + runtime.GOARCH
}
