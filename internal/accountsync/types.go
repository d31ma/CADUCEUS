// Package accountsync owns durable synchronization and remote-mutation
// orchestration for optional external accounts. Provider protocols and FYLO
// persistence implement the ports declared here.
package accountsync

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

var (
	ErrLeaseHeld          = errors.New("external account sync lease is held")
	ErrCheckpointConflict = errors.New("external account sync checkpoint conflict")
	ErrAmbiguousOutcome   = errors.New("external account remote mutation outcome is ambiguous")
	ErrInvalidBatch       = errors.New("invalid external account sync batch")
	ErrWatchUnsupported   = errors.New("external account change watch is unsupported")
	mutationID            = regexp.MustCompile(`^mut_[A-Za-z0-9_-]{20,80}$`)
)

const (
	MaxBatchFolders  = 250
	MaxBatchMessages = 500
	MaxBatchDeletes  = 500
	MaxBatchUpdates  = 500
	MaxBatchWindows  = 50
	MaxRawBatchBytes = 25 << 20
)

type Scope struct {
	Owner     accounts.Owner     `json:"-"`
	AccountID accounts.AccountID `json:"accountId"`
}

type Lease struct {
	Scope     Scope
	Token     string
	Holder    string
	ExpiresAt time.Time
}

type Checkpoint struct {
	Cursor    string
	Version   uint64
	UpdatedAt time.Time
}

type Folder struct {
	StableID   string   `json:"stableId"`
	Revision   string   `json:"revision"`
	Name       string   `json:"name"`
	Role       string   `json:"role"`
	Delimiter  string   `json:"delimiter"`
	Attributes []string `json:"attributes"`
	// ResetMessages is an internal epoch boundary (for example IMAP
	// UIDVALIDITY changing). It is acted on while committing the folder and is
	// never exposed in the persisted/provider API projection.
	ResetMessages bool `json:"-"`
}

// Message is the bounded provider index projection. RawRFC822 is transferred
// directly to a FYLO bucket by the persistence adapter and is never JSON
// encoded into an account document.
type Message struct {
	StableID        string    `json:"stableId"`
	RemoteStableID  string    `json:"-"`
	Revision        string    `json:"revision"`
	FolderStableID  string    `json:"folderStableId"`
	FolderStableIDs []string  `json:"folderStableIds,omitempty"`
	RFCMessageID    string    `json:"rfcMessageId"`
	InReplyTo       string    `json:"inReplyTo"`
	References      []string  `json:"references"`
	Subject         string    `json:"subject"`
	Sender          string    `json:"sender"`
	To              []string  `json:"to"`
	CC              []string  `json:"cc"`
	BCC             []string  `json:"bcc"`
	Snippet         string    `json:"snippet"`
	AttachmentNames []string  `json:"attachmentNames"`
	ReceivedAt      time.Time `json:"receivedAt"`
	Flags           []string  `json:"flags"`
	Size            uint64    `json:"size"`
	RawRFC822       []byte    `json:"-"`
}

// UnifiedMessage is the redacted provider-mail projection consumed by normal
// mailbox surfaces. It contains no credentials, raw source, or bucket ids.
type UnifiedMessage struct {
	Message
	AccountID      accounts.AccountID `json:"accountId"`
	MailboxAddress string             `json:"mailboxAddress"`
	AccountLabel   string             `json:"accountLabel"`
	AccountColor   string             `json:"accountColor"`
	FolderName     string             `json:"folderName"`
	FolderRole     string             `json:"folderRole"`
	Labels         []string           `json:"labels,omitempty"`
}

type UnifiedMessageFilter struct {
	AccountID      accounts.AccountID
	FolderRole     string
	FolderStableID string
	Query          string
	Read           *bool
	Starred        *bool
	ReceivedAtFrom *time.Time
	OldestFirst    bool
	Limit          int
	Offset         int
}

type Deletion struct {
	StableID       string `json:"stableId"`
	RemoteStableID string `json:"-"`
	Revision       string `json:"revision"`
}

// MessageUpdate changes mutable provider metadata without rewriting the raw
// RFC 822 bucket object. StableID continues to identify the immutable provider
// UID tuple while Revision makes replay idempotent.
type MessageUpdate struct {
	StableID        string   `json:"stableId"`
	RemoteStableID  string   `json:"-"`
	Revision        string   `json:"revision"`
	FolderStableIDs []string `json:"folderStableIds,omitempty"`
	Flags           []string `json:"flags"`
}

// ReconcileWindow is a complete, bounded provider UID interval. Persistence
// removes indexed messages in the interval that are absent from PresentIDs.
// This detects remote expunges without retaining an unbounded UID set in the
// durable cursor.
type ReconcileWindow struct {
	FolderStableID string   `json:"folderStableId"`
	UIDValidity    uint64   `json:"uidValidity"`
	StartUID       uint64   `json:"startUid"`
	EndUID         uint64   `json:"endUid"`
	PresentIDs     []string `json:"presentIds"`
}

type Batch struct {
	CursorBefore   string
	CursorAfter    string
	Phase          accounts.SyncPhase
	Completed      uint64
	Total          *uint64
	Folders        []Folder
	FolderSnapshot bool
	Messages       []Message
	Updates        []MessageUpdate
	Deletions      []Deletion
	Windows        []ReconcileWindow
}

type SyncRequest struct {
	Account     accounts.ExternalAccount
	Credentials accounts.Credentials
	Checkpoint  Checkpoint
}

type ChangeSink interface {
	Commit(context.Context, Batch) error
}

type MutationKind string
type MutationState string
type MutationOutcome string

const (
	MutationSetRead      MutationKind = "set_read"
	MutationSetStarred   MutationKind = "set_starred"
	MutationMove         MutationKind = "move"
	MutationDelete       MutationKind = "delete"
	MutationFolderCreate MutationKind = "folder_create"
	MutationFolderRename MutationKind = "folder_rename"
	MutationFolderDelete MutationKind = "folder_delete"

	MutationQueued      MutationState = "queued"
	MutationApplying    MutationState = "applying"
	MutationReconciling MutationState = "reconciling"
	MutationApplied     MutationState = "applied"
	MutationFailed      MutationState = "failed"

	OutcomeApplied    MutationOutcome = "applied"
	OutcomeNotApplied MutationOutcome = "not_applied"
	OutcomeUnknown    MutationOutcome = "unknown"
)

type MutationRequest struct {
	ID               string       `json:"id"`
	IdempotencyKey   string       `json:"idempotencyKey"`
	Kind             MutationKind `json:"kind"`
	MessageStableIDs []string     `json:"messageStableIds"`
	TargetFolderID   string       `json:"targetFolderId,omitempty"`
	FolderStableID   string       `json:"folderStableId,omitempty"`
	ParentStableID   string       `json:"parentStableId,omitempty"`
	FolderName       string       `json:"folderName,omitempty"`
	FolderPath       string       `json:"folderPath,omitempty"`
	NewFolderPath    string       `json:"newFolderPath,omitempty"`
	Value            bool         `json:"value"`
}

type Mutation struct {
	MutationRequest
	Scope           Scope         `json:"-"`
	State           MutationState `json:"state"`
	Attempts        uint16        `json:"attempts"`
	ResultStableIDs []string      `json:"resultStableIds,omitempty"`
	NotBefore       *time.Time    `json:"notBefore,omitempty"`
	LastCode        string        `json:"lastCode,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

type MutationContext struct {
	Account     accounts.ExternalAccount
	Credentials accounts.Credentials
	Mutation    Mutation
}

type MutationResult struct {
	Outcome         MutationOutcome
	Code            string
	RetryAfter      time.Duration
	ResultStableIDs []string
}

type Connector interface {
	Sync(context.Context, SyncRequest, ChangeSink) error
	ApplyMutation(context.Context, MutationContext) (MutationResult, error)
	ReconcileMutation(context.Context, MutationContext) (MutationResult, error)
}

// ChangeWatcher is an optimization port. A true result wakes the durable
// scheduler; false or ErrWatchUnsupported leaves ordinary polling in charge.
// Implementations must bound every provider connection and honor cancellation.
type ChangeWatcher interface {
	WaitForChange(context.Context, accounts.ExternalAccount, accounts.Credentials) (bool, error)
}

type Store interface {
	AcquireLease(context.Context, Scope, string, time.Time, time.Duration) (Lease, error)
	ReleaseLease(context.Context, Lease) error
	Checkpoint(context.Context, Scope) (Checkpoint, error)
	CommitBatch(context.Context, Lease, uint64, Batch) (Checkpoint, error)
	EnqueueMutation(context.Context, Scope, Mutation) (Mutation, error)
	MutationByIdempotencyKey(context.Context, Scope, string) (Mutation, bool, error)
	Mutation(context.Context, Scope, string) (Mutation, error)
	PendingMutations(context.Context, Lease, int) ([]Mutation, error)
	UpdateMutation(context.Context, Lease, Mutation) error
	ApplyMutationProjection(context.Context, Lease, Mutation) error
	DeleteAccountData(context.Context, Lease) error
}

type ScheduledAccount struct {
	Scope     Scope
	Provider  accounts.Provider
	NextRunAt time.Time
	UpdatedAt time.Time
	Attempts  uint16
	LastCode  string
}

type ScheduleStore interface {
	DueAccounts(context.Context, time.Time, int) ([]ScheduledAccount, error)
	Reschedule(context.Context, ScheduledAccount) error
}

// WatchScheduleSource exposes bounded credential-free account identities to
// the optional provider change watcher. It does not expose sync cursors or
// permit cross-owner mailbox reads.
type WatchScheduleSource interface {
	WatchAccounts(context.Context, int) ([]ScheduledAccount, error)
}

type Diagnostics struct {
	AccountID         accounts.AccountID `json:"accountId"`
	CursorVersion     uint64             `json:"cursorVersion"`
	CursorUpdatedAt   *time.Time         `json:"cursorUpdatedAt"`
	NextRunAt         time.Time          `json:"nextRunAt"`
	Attempts          uint16             `json:"attempts"`
	LastCode          string             `json:"lastCode,omitempty"`
	ScheduleUpdatedAt time.Time          `json:"scheduleUpdatedAt"`
}

type ScheduleControl interface {
	Retry(context.Context, Scope) error
	Diagnostics(context.Context, Scope) (Diagnostics, error)
}

type IndexReader interface {
	ListFolders(context.Context, Scope) ([]Folder, error)
	ListMessages(context.Context, Scope, string, int) ([]Message, error)
}

type UnifiedIndexReader interface {
	ListUnifiedMessages(context.Context, accounts.Owner, UnifiedMessageFilter) ([]UnifiedMessage, error)
	GetMessage(context.Context, Scope, string) (Message, error)
	GetMessageSource(context.Context, Scope, string) (Message, []byte, error)
}

// NewMailNotification is the bounded, credential-free projection placed in
// the scheduler-owned notification outbox when an incremental provider sync
// discovers unread Inbox mail. Initial account hydration never enters this
// queue.
type NewMailNotification struct {
	ID            string             `json:"id"`
	AccountID     accounts.AccountID `json:"accountId"`
	OwnerAddress  string             `json:"ownerAddress"`
	StableID      string             `json:"stableId"`
	Sender        string             `json:"sender"`
	Subject       string             `json:"subject"`
	ReceivedAt    time.Time          `json:"receivedAt"`
	Attempt       uint16             `json:"attempt"`
	DeliveryToken string             `json:"-"`
}

type NewMailNotificationClaim struct {
	Token         string                `json:"token"`
	Notifications []NewMailNotification `json:"notifications"`
}

// NotificationQueue is intentionally an internal workload port. Public
// mailbox callers use the owner-authorized unified index; only the signed
// notification worker may claim or settle cross-owner delivery work.
type NotificationQueue interface {
	ClaimNewMailNotifications(context.Context, string, time.Time, time.Duration, int) (NewMailNotificationClaim, error)
	CompleteNewMailNotifications(context.Context, string, []string, bool, string, time.Time) error
}

type MutationEnqueuer interface {
	EnqueueMutation(context.Context, accounts.Owner, accounts.ExternalAccount, MutationRequest) (Mutation, error)
	MutationByIdempotencyKey(context.Context, accounts.Owner, accounts.ExternalAccount, string) (Mutation, bool, error)
	Mutation(context.Context, accounts.Owner, accounts.ExternalAccount, string) (Mutation, error)
}

type AccountSource interface {
	Get(context.Context, accounts.Owner, accounts.AccountID) (accounts.ExternalAccount, error)
}

type AccountDeletionSource interface {
	FinalizeDelete(context.Context, accounts.Owner, accounts.AccountID) error
}

type AccountRunner interface {
	RunAccount(context.Context, accounts.Owner, accounts.ExternalAccount) error
	RunMutations(context.Context, accounts.Owner, accounts.ExternalAccount) error
	CleanupAccount(context.Context, accounts.Owner, accounts.ExternalAccount) error
}

// AccountJob shares the worker's exact account lease and short-lived decrypted
// credentials. Durable external submission uses this hook so synchronization,
// remote mutations, and SMTP delivery cannot race one account's lifecycle.
type AccountJob interface {
	RunWithLease(context.Context, Lease, accounts.ExternalAccount, accounts.Credentials) error
}

type CredentialsSource interface {
	Credentials(context.Context, accounts.ExternalAccount) (accounts.Credentials, error)
}

type Lifecycle interface {
	BeginSync(context.Context, accounts.Owner, accounts.AccountID, uint64) (accounts.ExternalAccount, error)
	ReportProgress(context.Context, accounts.Owner, accounts.AccountID, uint64, accounts.SyncProgressUpdate) (accounts.ExternalAccount, error)
	CompleteSync(context.Context, accounts.Owner, accounts.AccountID, uint64) (accounts.ExternalAccount, error)
	RecordFailure(context.Context, accounts.Owner, accounts.AccountID, uint64, accounts.SyncFailure) (accounts.ExternalAccount, error)
}

type FailureKind string

const (
	FailureAuthentication FailureKind = "authentication"
	FailureThrottled      FailureKind = "throttled"
	FailurePermanent      FailureKind = "permanent"
	FailureTransient      FailureKind = "transient"
)

// ConnectorError contains only a stable code and safe display text. Protocol
// banners, server errors, and credential material must never be wrapped in it.
type ConnectorError struct {
	Kind       FailureKind
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (failure ConnectorError) Error() string { return failure.Code + ": " + failure.Message }

func ValidateBatch(batch Batch) error {
	if len(batch.CursorBefore) > 4096 || batch.CursorAfter == "" || len(batch.CursorAfter) > 4096 ||
		len(batch.Folders) > MaxBatchFolders || len(batch.Messages) > MaxBatchMessages || len(batch.Updates) > MaxBatchUpdates ||
		len(batch.Deletions) > MaxBatchDeletes || len(batch.Windows) > MaxBatchWindows {
		return ErrInvalidBatch
	}
	if _, valid := syncPhase(batch.Phase); !valid || batch.Phase == accounts.PhaseComplete || batch.Total != nil && *batch.Total < batch.Completed {
		return ErrInvalidBatch
	}
	if batch.FolderSnapshot && batch.Phase != accounts.PhaseFolders {
		return ErrInvalidBatch
	}
	rawBytes := 0
	seenFolders := map[string]bool{}
	seenMessages := map[string]bool{}
	for _, folder := range batch.Folders {
		if invalidStable(folder.StableID, 512) || folder.Name == "" || len(folder.Name) > 1024 || len(folder.Revision) > 512 || seenFolders[folder.StableID] {
			return ErrInvalidBatch
		}
		seenFolders[folder.StableID] = true
	}
	for _, message := range batch.Messages {
		if invalidStable(message.StableID, 512) || message.RemoteStableID != "" && invalidStable(message.RemoteStableID, 512) || invalidStable(message.FolderStableID, 512) ||
			invalidFolderMemberships(message.FolderStableID, message.FolderStableIDs) || len(message.Revision) > 512 || seenMessages[message.StableID] ||
			len(message.Subject) > 998 || len(message.Sender) > 512 || len(message.Snippet) > 4096 || message.ReceivedAt.IsZero() ||
			invalidMessageValues(message.To, 100, 512) || invalidMessageValues(message.CC, 100, 512) || invalidMessageValues(message.BCC, 100, 512) ||
			len(message.InReplyTo) > 998 || invalidMessageValues(message.References, 100, 998) || invalidMessageValues(message.AttachmentNames, 100, 255) {
			return ErrInvalidBatch
		}
		seenMessages[message.StableID] = true
		rawBytes += len(message.RawRFC822)
		if len(message.RawRFC822) == 0 || uint64(len(message.RawRFC822)) != message.Size || rawBytes > MaxRawBatchBytes {
			return ErrInvalidBatch
		}
	}
	for _, update := range batch.Updates {
		if invalidStable(update.StableID, 512) || update.RemoteStableID != "" && invalidStable(update.RemoteStableID, 512) || invalidStableList(update.FolderStableIDs, MaxBatchFolders, 512) || len(update.Revision) > 512 || seenMessages[update.StableID] || invalidFlags(update.Flags) {
			return ErrInvalidBatch
		}
		seenMessages[update.StableID] = true
	}
	for _, deletion := range batch.Deletions {
		if invalidStable(deletion.StableID, 512) || deletion.RemoteStableID != "" && invalidStable(deletion.RemoteStableID, 512) || seenMessages[deletion.StableID] {
			return ErrInvalidBatch
		}
		seenMessages[deletion.StableID] = true
	}
	windowIDs := 0
	for _, window := range batch.Windows {
		if invalidStable(window.FolderStableID, 512) || window.UIDValidity == 0 || window.StartUID == 0 || window.EndUID < window.StartUID {
			return ErrInvalidBatch
		}
		seen := map[string]bool{}
		for _, id := range window.PresentIDs {
			if invalidStable(id, 512) || seen[id] {
				return ErrInvalidBatch
			}
			seen[id] = true
			windowIDs++
		}
	}
	if windowIDs > MaxBatchMessages {
		return ErrInvalidBatch
	}
	return nil
}

func invalidStableList(values []string, limit, length int) bool {
	if len(values) > limit {
		return true
	}
	seen := map[string]bool{}
	for _, value := range values {
		if invalidStable(value, length) || seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func invalidFolderMemberships(primary string, memberships []string) bool {
	if len(memberships) == 0 {
		return false
	}
	if len(memberships) > MaxBatchFolders {
		return true
	}
	seen, primaryFound := map[string]bool{}, false
	for _, folder := range memberships {
		if invalidStable(folder, 512) || seen[folder] {
			return true
		}
		seen[folder] = true
		primaryFound = primaryFound || folder == primary
	}
	return !primaryFound
}

func invalidMessageValues(values []string, maximum, length int) bool {
	if len(values) > maximum {
		return true
	}
	for _, value := range values {
		if invalidStable(value, length) {
			return true
		}
	}
	return false
}

func invalidFlags(flags []string) bool {
	if len(flags) > 100 {
		return true
	}
	for _, flag := range flags {
		if invalidStable(flag, 128) {
			return true
		}
	}
	return false
}

func ValidateMutation(request MutationRequest) error {
	if !mutationID.MatchString(request.ID) || strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 || len(request.MessageStableIDs) > 100 {
		return fmt.Errorf("%w: mutation identity", ErrInvalidBatch)
	}
	switch request.Kind {
	case MutationSetRead, MutationSetStarred, MutationDelete:
		if len(request.MessageStableIDs) == 0 || request.TargetFolderID != "" || request.FolderStableID != "" || request.ParentStableID != "" || request.FolderName != "" || request.FolderPath != "" || request.NewFolderPath != "" {
			return fmt.Errorf("%w: mutation target", ErrInvalidBatch)
		}
	case MutationMove:
		if len(request.MessageStableIDs) == 0 || invalidStable(request.TargetFolderID, 512) || request.FolderStableID != "" || request.ParentStableID != "" || request.FolderName != "" || request.FolderPath != "" || request.NewFolderPath != "" {
			return fmt.Errorf("%w: mutation target", ErrInvalidBatch)
		}
	case MutationFolderCreate:
		if len(request.MessageStableIDs) != 0 || request.TargetFolderID != "" || request.FolderStableID != "" || request.FolderPath != "" || invalidStable(request.NewFolderPath, 1024) || invalidFolderName(request.FolderName) || request.ParentStableID != "" && invalidStable(request.ParentStableID, 512) {
			return fmt.Errorf("%w: folder mutation target", ErrInvalidBatch)
		}
	case MutationFolderRename:
		if len(request.MessageStableIDs) != 0 || request.TargetFolderID != "" || invalidStable(request.FolderStableID, 512) || request.ParentStableID != "" || invalidFolderName(request.FolderName) || invalidStable(request.FolderPath, 1024) || invalidStable(request.NewFolderPath, 1024) || request.FolderPath == request.NewFolderPath {
			return fmt.Errorf("%w: folder mutation target", ErrInvalidBatch)
		}
	case MutationFolderDelete:
		if len(request.MessageStableIDs) != 0 || request.TargetFolderID != "" || invalidStable(request.FolderStableID, 512) || request.ParentStableID != "" || request.FolderName != "" || invalidStable(request.FolderPath, 1024) || request.NewFolderPath != "" {
			return fmt.Errorf("%w: folder mutation target", ErrInvalidBatch)
		}
	default:
		return fmt.Errorf("%w: mutation kind", ErrInvalidBatch)
	}
	seen := map[string]bool{}
	for _, id := range request.MessageStableIDs {
		if invalidStable(id, 512) || seen[id] {
			return fmt.Errorf("%w: message identity", ErrInvalidBatch)
		}
		seen[id] = true
	}
	return nil
}

// ValidateMutationResults keeps provider-assigned identities ordered with the
// original message identities. Only a move may replace an IMAP stable ID.
func ValidateMutationResults(mutation Mutation) error {
	if len(mutation.ResultStableIDs) == 0 {
		return nil
	}
	if mutation.Kind != MutationMove || len(mutation.ResultStableIDs) != len(mutation.MessageStableIDs) {
		return fmt.Errorf("%w: mutation result identity", ErrInvalidBatch)
	}
	seen := make(map[string]bool, len(mutation.ResultStableIDs))
	for _, id := range mutation.ResultStableIDs {
		if invalidStable(id, 512) || seen[id] {
			return fmt.Errorf("%w: mutation result identity", ErrInvalidBatch)
		}
		seen[id] = true
	}
	return nil
}

func IsFolderMutation(kind MutationKind) bool {
	return kind == MutationFolderCreate || kind == MutationFolderRename || kind == MutationFolderDelete
}

func invalidFolderName(value string) bool {
	return strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || len(value) > 128 || strings.ContainsAny(value, "\x00\r\n")
}

func invalidStable(value string, limit int) bool {
	return strings.TrimSpace(value) == "" || len(value) > limit || strings.ContainsAny(value, "\x00\r\n")
}

func syncPhase(phase accounts.SyncPhase) (int, bool) {
	for index, candidate := range []accounts.SyncPhase{accounts.PhaseConnecting, accounts.PhaseFolders, accounts.PhaseHeaders, accounts.PhaseBodies, accounts.PhaseContacts, accounts.PhaseCalendar} {
		if phase == candidate {
			return index, true
		}
	}
	return 0, false
}
