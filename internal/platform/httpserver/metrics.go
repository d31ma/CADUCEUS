package httpserver

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type metricKey struct {
	route  string
	method string
	status int
}

// Metrics stores bounded-cardinality process and HTTP measurements.
type Metrics struct {
	inFlight atomic.Int64
	mu       sync.Mutex
	requests map[metricKey]uint64
	duration map[metricKey]time.Duration
}

func newMetrics() *Metrics {
	return &Metrics{
		requests: make(map[metricKey]uint64),
		duration: make(map[metricKey]time.Duration),
	}
}

func (metrics *Metrics) begin() {
	metrics.inFlight.Add(1)
}

func (metrics *Metrics) end(route, method string, status int, duration time.Duration) {
	metrics.inFlight.Add(-1)
	key := metricKey{route: route, method: method, status: status}
	metrics.mu.Lock()
	metrics.requests[key]++
	metrics.duration[key] += duration
	metrics.mu.Unlock()
}

func (metrics *Metrics) writePrometheus(output io.Writer, ready bool) {
	_, _ = fmt.Fprintln(output, "# HELP caduceus_ready Whether the process is ready to receive traffic.")
	_, _ = fmt.Fprintln(output, "# TYPE caduceus_ready gauge")
	if ready {
		_, _ = fmt.Fprintln(output, "caduceus_ready 1")
	} else {
		_, _ = fmt.Fprintln(output, "caduceus_ready 0")
	}
	_, _ = fmt.Fprintln(output, "# HELP caduceus_http_in_flight_requests Current in-flight HTTP requests.")
	_, _ = fmt.Fprintln(output, "# TYPE caduceus_http_in_flight_requests gauge")
	_, _ = fmt.Fprintf(output, "caduceus_http_in_flight_requests %d\n", metrics.inFlight.Load())
	_, _ = fmt.Fprintln(output, "# HELP caduceus_go_goroutines Current Go goroutine count.")
	_, _ = fmt.Fprintln(output, "# TYPE caduceus_go_goroutines gauge")
	_, _ = fmt.Fprintf(output, "caduceus_go_goroutines %d\n", runtime.NumGoroutine())

	metrics.mu.Lock()
	keys := make([]metricKey, 0, len(metrics.requests))
	for key := range metrics.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := keys[i].route + keys[i].method + strconv.Itoa(keys[i].status)
		right := keys[j].route + keys[j].method + strconv.Itoa(keys[j].status)
		return left < right
	})
	requests := make(map[metricKey]uint64, len(metrics.requests))
	durations := make(map[metricKey]time.Duration, len(metrics.duration))
	for _, key := range keys {
		requests[key] = metrics.requests[key]
		durations[key] = metrics.duration[key]
	}
	metrics.mu.Unlock()

	_, _ = fmt.Fprintln(output, "# HELP caduceus_http_requests_total HTTP requests by normalized route, method, and status.")
	_, _ = fmt.Fprintln(output, "# TYPE caduceus_http_requests_total counter")
	for _, key := range keys {
		labels := metricLabels(key)
		_, _ = fmt.Fprintf(output, "caduceus_http_requests_total%s %d\n", labels, requests[key])
	}
	_, _ = fmt.Fprintln(output, "# HELP caduceus_http_request_duration_seconds_total Cumulative HTTP request duration.")
	_, _ = fmt.Fprintln(output, "# TYPE caduceus_http_request_duration_seconds_total counter")
	for _, key := range keys {
		labels := metricLabels(key)
		_, _ = fmt.Fprintf(output, "caduceus_http_request_duration_seconds_total%s %.6f\n", labels, durations[key].Seconds())
	}
}

func metricLabels(key metricKey) string {
	return fmt.Sprintf(
		`{route=%q,method=%q,status=%q}`,
		key.route,
		key.method,
		strconv.Itoa(key.status),
	)
}
