package auth

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Authentication policy. Both modes require multi-factor assurance before a
// session is issued; they differ in whether a password is one of the factors.
const (
	// PolicyPasskeyFirst accepts a user-verified passkey on its own. It is
	// phishing-resistant and establishes MFA by itself.
	PolicyPasskeyFirst = "passkey-first"
	// PolicyPasswordAndPasskey additionally requires the password factor, for
	// deployments that want a memorized secret in the ladder.
	PolicyPasswordAndPasskey = "password-and-passkey"
)

// transactionLifetime mirrors the engine's own bound. A ledger entry that
// outlives the transaction it describes is only a leak.
const transactionLifetime = 10 * time.Minute

// factorLedger records which factors one in-flight transaction has actually
// satisfied.
//
// SESAME reports the assurance a transaction has reached but not the path it
// took there, and the two policies differ precisely on the path: a
// user-verified passkey reaches `mfa` on its own, so "password and passkey"
// cannot be enforced from the assurance value alone. Entries are bounded by
// the engine's own transaction lifetime and dropped on completion.
type factorLedger struct {
	mu      sync.Mutex
	entries map[string]ledgerEntry
}

type ledgerEntry struct {
	factors map[string]struct{}
	expires time.Time
}

func newFactorLedger() *factorLedger {
	return &factorLedger{entries: map[string]ledgerEntry{}}
}

func (ledger *factorLedger) record(transactionID, factor string, now time.Time) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.evictLocked(now)
	entry, found := ledger.entries[transactionID]
	if !found {
		entry = ledgerEntry{factors: map[string]struct{}{}}
	}
	entry.factors[factor] = struct{}{}
	entry.expires = now.Add(transactionLifetime)
	ledger.entries[transactionID] = entry
}

func (ledger *factorLedger) verified(transactionID, factor string, now time.Time) bool {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	entry, found := ledger.entries[transactionID]
	if !found || !entry.expires.After(now) {
		return false
	}
	_, satisfied := entry.factors[factor]
	return satisfied
}

func (ledger *factorLedger) forget(transactionID string) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	delete(ledger.entries, transactionID)
}

func (ledger *factorLedger) evictLocked(now time.Time) {
	for id, entry := range ledger.entries {
		if !entry.expires.After(now) {
			delete(ledger.entries, id)
		}
	}
}

// ValidatePolicy rejects an unknown authentication policy at load time rather
// than at the first login.
func ValidatePolicy(policy string) error {
	switch policy {
	case "", PolicyPasskeyFirst, PolicyPasswordAndPasskey:
		return nil
	default:
		return fmt.Errorf(
			"CADUCEUS_AUTH_POLICY must be %s or %s",
			PolicyPasskeyFirst, PolicyPasswordAndPasskey,
		)
	}
}

// ErrPolicy reports a transaction that has not met the deployment's
// authentication policy.
var ErrPolicy = errors.New("authentication policy is not satisfied")

// requiredFactors lists what a caller must satisfy under one policy. It is
// returned by login begin so a client can present the right prompts without
// encoding the policy itself.
func requiredFactors(policy string) []string {
	if policy == PolicyPasswordAndPasskey {
		return []string{FactorPassword, FactorPasskey}
	}
	return []string{FactorPasskey}
}

// Factor names as CADUCEUS reports them to clients. They match the engine's
// vocabulary so one word means one thing across the stack.
const (
	FactorPassword     = "password"
	FactorPasskey      = "passkey"
	FactorTOTP         = "totp"
	FactorRecoveryCode = "recovery_code"
)

// Assurance levels reported by the engine.
const (
	AssurancePassword = "password"
	AssuranceMFA      = "mfa"
)

// Transaction states reported by the engine.
const (
	StateStarted        = "started"
	StateAwaitingFactor = "awaiting_factor"
	StateCompleted      = "completed"
	StateFailed         = "failed"
)
