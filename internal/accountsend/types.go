// Package accountsend owns durable outbound submission for optional external
// accounts. Provider transports and FYLO persistence implement the ports here;
// callers never supply credentials or connection endpoints.
package accountsend

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

const (
	MaxRecipients = 100
	MaxRawBytes   = 25 << 20
	MaxPending    = 20
)

var (
	ErrInvalidSubmission  = errors.New("invalid external account submission")
	ErrSubmissionConflict = errors.New("external account submission conflict")
	submissionID          = regexp.MustCompile(`^sub_[A-Za-z0-9_-]{20,80}$`)
)

type State string
type Outcome string

const (
	StateBuilding  State = "building"
	StateEditing   State = "editing"
	StateQueued    State = "queued"
	StateApplying  State = "applying"
	StateSent      State = "sent"
	StateFailed    State = "failed"
	StateAmbiguous State = "ambiguous"
	StateCancelled State = "cancelled"

	OutcomeSent      Outcome = "sent"
	OutcomeRetry     Outcome = "retry"
	OutcomeRejected  Outcome = "rejected"
	OutcomeAmbiguous Outcome = "ambiguous"
)

type Delivery struct {
	Recipient string
	RawRFC822 []byte
}

type Request struct {
	ID             string
	IdempotencyKey string
	EnvelopeFrom   string
	Recipients     []string
	RawRFC822      []byte
	NotBefore      *time.Time
}

type BatchRequest struct {
	ID             string
	IdempotencyKey string
	EnvelopeFrom   string
	Recipients     []string
	NotBefore      *time.Time
}

type AppendRequest struct {
	ID        string
	Revision  uint64
	Index     int
	Recipient string
	RawRFC822 []byte
}

type Submission struct {
	ID             string            `json:"id"`
	IdempotencyKey string            `json:"idempotencyKey"`
	Revision       uint64            `json:"revision"`
	Scope          accountsync.Scope `json:"-"`
	EnvelopeFrom   string            `json:"envelopeFrom"`
	Recipients     []string          `json:"recipients"`
	RawRFC822      []byte            `json:"-"`
	Deliveries     []Delivery        `json:"-"`
	DeliveryCount  int               `json:"deliveryCount,omitempty"`
	State          State             `json:"state"`
	Attempts       uint16            `json:"attempts"`
	NotBefore      *time.Time        `json:"notBefore,omitempty"`
	LastCode       string            `json:"lastCode,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

type Result struct {
	Outcome    Outcome
	Code       string
	RetryAfter time.Duration
}

type Context struct {
	Account     accounts.ExternalAccount
	Credentials accounts.Credentials
	Submission  Submission
}

type Connector interface {
	Submit(context.Context, Context) (Result, error)
}

type Store interface {
	EnqueueSubmission(context.Context, accountsync.Scope, Submission) (Submission, error)
	BeginSubmissionBatch(context.Context, accountsync.Scope, Submission) (Submission, error)
	AppendSubmissionDelivery(context.Context, accountsync.Scope, AppendRequest) (Submission, error)
	CommitSubmissionBatch(context.Context, accountsync.Scope, string, uint64) (Submission, error)
	BeginSubmissionBatchReplacement(context.Context, accountsync.Scope, BatchRequest, uint64) (Submission, error)
	AppendSubmissionReplacementDelivery(context.Context, accountsync.Scope, AppendRequest) (Submission, error)
	CommitSubmissionBatchReplacement(context.Context, accountsync.Scope, string, uint64) (Submission, error)
	AbortSubmissionBatchReplacement(context.Context, accountsync.Scope, string, uint64) (Submission, error)
	ReplaceSubmission(context.Context, accountsync.Scope, Request, uint64) (Submission, error)
	PendingSubmissions(context.Context, accountsync.Lease, int) ([]Submission, error)
	UpdateSubmission(context.Context, accountsync.Lease, Submission) error
	CancelSubmission(context.Context, accountsync.Scope, string) (Submission, error)
	GetSubmission(context.Context, accountsync.Scope, string) (Submission, error)
}

type Reader interface {
	GetSubmission(context.Context, accountsync.Scope, string) (Submission, error)
}

type Enqueuer interface {
	Enqueue(context.Context, accounts.Owner, accounts.ExternalAccount, Request) (Submission, error)
	BeginBatch(context.Context, accounts.Owner, accounts.ExternalAccount, BatchRequest) (Submission, error)
	AppendBatch(context.Context, accounts.Owner, accounts.ExternalAccount, AppendRequest) (Submission, error)
	CommitBatch(context.Context, accounts.Owner, accounts.ExternalAccount, string, uint64) (Submission, error)
	BeginBatchReplace(context.Context, accounts.Owner, accounts.ExternalAccount, BatchRequest, uint64) (Submission, error)
	AppendBatchReplace(context.Context, accounts.Owner, accounts.ExternalAccount, AppendRequest) (Submission, error)
	CommitBatchReplace(context.Context, accounts.Owner, accounts.ExternalAccount, string, uint64) (Submission, error)
	AbortBatchReplace(context.Context, accounts.Owner, accounts.ExternalAccount, string, uint64) (Submission, error)
	Replace(context.Context, accounts.Owner, accounts.ExternalAccount, Request, uint64) (Submission, error)
	Cancel(context.Context, accounts.Owner, accounts.ExternalAccount, string) (Submission, error)
}

func ValidateBatchRequest(request BatchRequest, now time.Time) error {
	if !submissionID.MatchString(request.ID) || len(request.IdempotencyKey) < 1 || len(request.IdempotencyKey) > 128 ||
		invalidHeaderValue(request.IdempotencyKey) || len(request.Recipients) < 1 || len(request.Recipients) > MaxRecipients ||
		normalizeAddress(request.EnvelopeFrom) != request.EnvelopeFrom {
		return ErrInvalidSubmission
	}
	seen := make(map[string]bool, len(request.Recipients))
	for _, recipient := range request.Recipients {
		if normalizeAddress(recipient) != recipient || seen[recipient] {
			return ErrInvalidSubmission
		}
		seen[recipient] = true
	}
	if request.NotBefore != nil {
		when := request.NotBefore.UTC()
		if when.Before(now.Add(-time.Minute)) || when.After(now.Add(366*24*time.Hour)) {
			return ErrInvalidSubmission
		}
	}
	return nil
}

func ValidateAppendRequest(request AppendRequest) error {
	if !submissionID.MatchString(request.ID) || request.Revision < 1 || request.Index < 0 || request.Index >= MaxRecipients ||
		normalizeAddress(request.Recipient) != request.Recipient || len(request.RawRFC822) < 1 || len(request.RawRFC822) > MaxRawBytes {
		return ErrInvalidSubmission
	}
	return nil
}

func ValidateRequest(request Request, now time.Time) error {
	if !submissionID.MatchString(request.ID) || len(request.IdempotencyKey) < 1 || len(request.IdempotencyKey) > 128 ||
		invalidHeaderValue(request.IdempotencyKey) || len(request.RawRFC822) < 1 || len(request.RawRFC822) > MaxRawBytes ||
		len(request.Recipients) < 1 || len(request.Recipients) > MaxRecipients || normalizeAddress(request.EnvelopeFrom) != request.EnvelopeFrom {
		return ErrInvalidSubmission
	}
	seen := make(map[string]bool, len(request.Recipients))
	for _, recipient := range request.Recipients {
		if normalizeAddress(recipient) != recipient || seen[recipient] {
			return ErrInvalidSubmission
		}
		seen[recipient] = true
	}
	if request.NotBefore != nil {
		when := request.NotBefore.UTC()
		if when.Before(now.Add(-time.Minute)) || when.After(now.Add(366*24*time.Hour)) {
			return ErrInvalidSubmission
		}
	}
	return nil
}

func normalizeAddress(value string) string {
	trimmed := strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed || len(trimmed) > 254 || strings.ContainsAny(trimmed, "\r\n\x00") {
		return ""
	}
	parts := strings.Split(parsed.Address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return ""
	}
	return strings.ToLower(parsed.Address)
}

func invalidHeaderValue(value string) bool { return strings.ContainsAny(value, "\r\n\x00") }
