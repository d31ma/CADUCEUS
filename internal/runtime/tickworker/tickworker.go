// Package tickworker drives the API's durable background routes on a timer.
//
// It ports background/OutboundQueueWorker.ts, SnoozeWorker.ts,
// FollowUpReminderWorker.ts and ProviderNotificationWorker.ts. Each worker
// POSTs `{}` to one signed tick route of the local API, which stays the only
// FYLO writer; the worker owns scheduling, non-overlap, request signing and
// measurements, but no delivery state.
package tickworker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/d31ma/caduceus/internal/runtime/supervisor"
)

// Options configures one tick worker.
type Options struct {
	APIURL   string
	Secret   string
	Interval time.Duration
	Timeout  time.Duration
	Client   *http.Client
}

// Sign answers the X-Caduceus-Signature value for body.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type tick struct {
	worker *supervisor.PeriodicWorker
}

// Start begins the interval.
func (t *tick) Start() (supervisor.StopFunc, error) { return t.worker.Start() }

// Stop ends the interval and waits for an in-flight run.
func (t *tick) Stop(ctx context.Context) error { return t.worker.Stop(ctx) }

// RunNow runs one tick unless one is in flight.
func (t *tick) RunNow() bool { return t.worker.RunNow() }

func newTick(name, route string, options Options, defaultInterval time.Duration, failure string, onBody func([]byte) error) (*tick, error) {
	apiURL := strings.TrimSuffix(options.APIURL, "/")
	interval := options.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := options.Client
	if client == nil {
		client = http.DefaultClient
	}
	body := []byte("{}")
	worker, err := supervisor.NewPeriodicWorker(name, interval, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+route, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Caduceus-Signature", Sign(options.Secret, body))
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode > 299 {
			_, _ = io.Copy(io.Discard, response.Body)
			return fmt.Errorf("%s failed with HTTP %d", failure, response.StatusCode)
		}
		if onBody == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			return nil
		}
		payload, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		return onBody(payload)
	})
	if err != nil {
		return nil, err
	}
	return &tick{worker: worker}, nil
}

// Snooze drives `POST /snooze/tick`.
type Snooze struct{ *tick }

// NewSnooze returns the snooze-due worker (default interval 30 s).
func NewSnooze(options Options) (*Snooze, error) {
	t, err := newTick("snooze-due", "/snooze/tick", options, 30*time.Second, "Snooze tick", nil)
	if err != nil {
		return nil, err
	}
	return &Snooze{t}, nil
}

// Snapshot answers the worker metrics.
func (worker *Snooze) Snapshot() supervisor.WorkerSnapshot { return worker.worker.Snapshot() }

// ProviderNotification drives `POST /notifications/provider/tick`.
type ProviderNotification struct{ *tick }

// NewProviderNotification returns the provider-new-mail worker (default
// interval 15 s).
func NewProviderNotification(options Options) (*ProviderNotification, error) {
	t, err := newTick("provider-new-mail", "/notifications/provider/tick", options, 15*time.Second, "Provider notification tick", nil)
	if err != nil {
		return nil, err
	}
	return &ProviderNotification{t}, nil
}

// Snapshot answers the worker metrics.
func (worker *ProviderNotification) Snapshot() supervisor.WorkerSnapshot {
	return worker.worker.Snapshot()
}

// Delivery counts the outcome of the last outbound tick.
type Delivery struct {
	Delivered uint64 `json:"delivered"`
	Retried   uint64 `json:"retried"`
	Bounced   uint64 `json:"bounced"`
}

// QueueHealth is the outbound queue depth the API reports.
type QueueHealth struct {
	Depth       uint64 `json:"depth"`
	Queued      uint64 `json:"queued"`
	Sending     uint64 `json:"sending"`
	Due         uint64 `json:"due"`
	OldestAgeMs uint64 `json:"oldestAgeMs"`
	Attempts    uint64 `json:"attempts"`
}

// OutboundSnapshot is the outbound worker metrics plus the last tick result.
type OutboundSnapshot struct {
	supervisor.WorkerSnapshot
	LastDelivery *Delivery    `json:"lastDelivery,omitempty"`
	Queue        *QueueHealth `json:"queue,omitempty"`
}

// OutboundQueue drives `POST /send/tick`.
type OutboundQueue struct {
	*tick
	mu           sync.Mutex
	lastDelivery *Delivery
	queue        *QueueHealth
}

// NewOutboundQueue returns the outbound-queue worker (default interval 30 s).
func NewOutboundQueue(options Options) (*OutboundQueue, error) {
	worker := &OutboundQueue{}
	t, err := newTick("outbound-queue", "/send/tick", options, 30*time.Second, "Queue tick", func(payload []byte) error {
		var result map[string]any
		if err := decodeNumbers(payload, &result); err != nil {
			return err
		}
		delivery := &Delivery{
			Delivered: boundedCount(result["delivered"]),
			Retried:   boundedCount(result["retried"]),
			Bounced:   boundedCount(result["bounced"]),
		}
		health := queueHealth(result["queue"])
		worker.mu.Lock()
		worker.lastDelivery = delivery
		worker.queue = health
		worker.mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	worker.tick = t
	return worker, nil
}

// Snapshot answers the worker metrics and the last tick result.
func (worker *OutboundQueue) Snapshot() OutboundSnapshot {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return OutboundSnapshot{WorkerSnapshot: worker.worker.Snapshot(), LastDelivery: worker.lastDelivery, Queue: worker.queue}
}

// ReminderSnapshot is the reminder worker metrics plus the last tick result.
type ReminderSnapshot struct {
	supervisor.WorkerSnapshot
	LastResult json.RawMessage `json:"lastResult,omitempty"`
}

// FollowUpReminders drives `POST /reminders/tick`.
type FollowUpReminders struct {
	*tick
	mu         sync.Mutex
	lastResult json.RawMessage
}

// NewFollowUpReminders returns the follow-up-reminders worker (default
// interval 30 s).
func NewFollowUpReminders(options Options) (*FollowUpReminders, error) {
	worker := &FollowUpReminders{}
	t, err := newTick("follow-up-reminders", "/reminders/tick", options, 30*time.Second, "Reminder tick", func(payload []byte) error {
		var result any
		if err := decodeNumbers(payload, &result); err != nil {
			return err
		}
		if !isReminderResult(result) {
			return fmt.Errorf("Reminder tick returned an invalid result")
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, payload); err != nil {
			return err
		}
		worker.mu.Lock()
		worker.lastResult = compact.Bytes()
		worker.mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	worker.tick = t
	return worker, nil
}

// Snapshot answers the worker metrics and the last tick result.
func (worker *FollowUpReminders) Snapshot() ReminderSnapshot {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return ReminderSnapshot{WorkerSnapshot: worker.worker.Snapshot(), LastResult: worker.lastResult}
}

func decodeNumbers(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func isReminderResult(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"due", "surfaced", "reconciled", "failed"} {
		number, ok := object[key].(json.Number)
		if !ok {
			return false
		}
		count, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || count < 0 || count > maxSafeInteger || count != math.Trunc(count) {
			return false
		}
	}
	return true
}

const maxSafeInteger = 9007199254740991

// boundedCount ports the JavaScript `Number(value)` check: a safe,
// non-negative integer, or 0.
func boundedCount(value any) uint64 {
	var count float64
	switch typed := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil {
			return 0
		}
		count = parsed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0
		}
		count = parsed
	case bool:
		if typed {
			return 1
		}
		return 0
	default:
		return 0
	}
	if count < 0 || count > maxSafeInteger || count != math.Trunc(count) {
		return 0
	}
	return uint64(count)
}

func queueHealth(value any) *QueueHealth {
	switch typed := value.(type) {
	case map[string]any:
		return &QueueHealth{
			Depth: boundedCount(typed["depth"]), Queued: boundedCount(typed["queued"]), Sending: boundedCount(typed["sending"]),
			Due: boundedCount(typed["due"]), OldestAgeMs: boundedCount(typed["oldestAgeMs"]), Attempts: boundedCount(typed["attempts"]),
		}
	case []any:
		// JavaScript treats an array as an object here, with no named counts.
		return &QueueHealth{}
	default:
		return nil
	}
}
