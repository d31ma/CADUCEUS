// Package supervisor owns a group of process-level services as one lifecycle.
//
// It is the Go port of the former background/ServiceSupervisor.ts. Required startup
// failures roll back the services already started. Optional failures leave
// the runtime degraded but ready, which suits helpers such as certificate
// renewal that should not hide an otherwise usable server.
package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RuntimeState is the lifecycle state of the whole supervisor.
type RuntimeState string

// Runtime states, spelled as the JavaScript supervisor reported them.
const (
	StateIdle     RuntimeState = "idle"
	StateStarting RuntimeState = "starting"
	StateReady    RuntimeState = "ready"
	StateDegraded RuntimeState = "degraded"
	StateStopping RuntimeState = "stopping"
	StateStopped  RuntimeState = "stopped"
	StateFailed   RuntimeState = "failed"
)

// ComponentState is the lifecycle state of one supervised service.
type ComponentState string

// Component states, spelled as the JavaScript supervisor reported them.
const (
	ComponentPending  ComponentState = "pending"
	ComponentStarting ComponentState = "starting"
	ComponentReady    ComponentState = "ready"
	ComponentStopping ComponentState = "stopping"
	ComponentStopped  ComponentState = "stopped"
	ComponentFailed   ComponentState = "failed"
)

// StopFunc stops one started service.
type StopFunc func(context.Context) error

// Service describes one supervised service. A service is required unless
// Optional is set, matching the JavaScript default of `required !== false`.
type Service struct {
	Name     string
	Optional bool
	// Start brings the service up and answers how to stop it. A nil StopFunc
	// means the service has nothing to stop.
	Start func(context.Context) (StopFunc, error)
}

// ComponentSnapshot is the serializable state of one service.
type ComponentSnapshot struct {
	Required  bool           `json:"required"`
	State     ComponentState `json:"state"`
	StartedAt string         `json:"startedAt,omitempty"`
	StoppedAt string         `json:"stoppedAt,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// Components keeps the services in registration order when encoded, as the
// JavaScript `Object.fromEntries` did.
type Components struct {
	Names  []string
	ByName map[string]ComponentSnapshot
}

// MarshalJSON encodes the components as an object in registration order.
func (components Components) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, name := range components.Names {
		if index > 0 {
			buffer.WriteByte(',')
		}
		key, err := Marshal(name)
		if err != nil {
			return nil, err
		}
		value, err := Marshal(components.ByName[name])
		if err != nil {
			return nil, err
		}
		buffer.Write(key)
		buffer.WriteByte(':')
		buffer.Write(value)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

// Snapshot is the serializable state of the supervisor.
type Snapshot struct {
	State      RuntimeState `json:"state"`
	StartedAt  string       `json:"startedAt,omitempty"`
	ReadyAt    string       `json:"readyAt,omitempty"`
	StoppedAt  string       `json:"stoppedAt,omitempty"`
	Components Components   `json:"components"`
}

// Options configures a Supervisor.
type Options struct {
	// OnRequiredFailure runs, on its own goroutine, when a required service
	// reports a failure after startup.
	OnRequiredFailure func(name string, err error)
	// Now overrides the clock in tests.
	Now func() time.Time
}

type registered struct {
	service  Service
	snapshot ComponentSnapshot
	stop     StopFunc
}

// Supervisor starts services in registration order and stops them in reverse.
type Supervisor struct {
	mu        sync.Mutex
	services  []*registered
	options   Options
	state     RuntimeState
	startedAt string
	readyAt   string
	stoppedAt string
	stopOnce  sync.Once
	stopDone  chan struct{}
}

// New returns an idle supervisor.
func New(options Options) *Supervisor {
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Supervisor{options: options, state: StateIdle, stopDone: make(chan struct{})}
}

// Register adds a service. It must run before Start.
func (supervisor *Supervisor) Register(service Service) error {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.state != StateIdle {
		return errors.New("Services must be registered before the supervisor starts")
	}
	if strings.TrimSpace(service.Name) == "" {
		return errors.New("A supervised service requires a name")
	}
	for _, existing := range supervisor.services {
		if existing.service.Name == service.Name {
			return fmt.Errorf("Service %q is already registered", service.Name)
		}
	}
	supervisor.services = append(supervisor.services, &registered{
		service:  service,
		snapshot: ComponentSnapshot{Required: !service.Optional, State: ComponentPending},
	})
	return nil
}

// Start starts every service in registration order. A required failure stops
// the services already started and answers that failure.
func (supervisor *Supervisor) Start(ctx context.Context) error {
	supervisor.mu.Lock()
	if supervisor.state != StateIdle {
		state := supervisor.state
		supervisor.mu.Unlock()
		return fmt.Errorf("Cannot start services while supervisor is %s", state)
	}
	supervisor.state = StateStarting
	supervisor.startedAt = supervisor.stamp()
	services := append([]*registered(nil), supervisor.services...)
	supervisor.mu.Unlock()

	degraded := false
	for _, service := range services {
		supervisor.mu.Lock()
		service.snapshot.State = ComponentStarting
		supervisor.mu.Unlock()
		stop, err := supervisor.startOne(ctx, service.service)
		supervisor.mu.Lock()
		if err == nil {
			service.stop = stop
			service.snapshot.State = ComponentReady
			service.snapshot.StartedAt = supervisor.stamp()
			supervisor.mu.Unlock()
			continue
		}
		service.snapshot.State = ComponentFailed
		service.snapshot.Error = err.Error()
		if service.service.Optional {
			degraded = true
			supervisor.mu.Unlock()
			continue
		}
		supervisor.state = StateFailed
		supervisor.mu.Unlock()
		supervisor.stopStarted(ctx)
		return err
	}

	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	supervisor.readyAt = supervisor.stamp()
	if degraded {
		supervisor.state = StateDegraded
	} else {
		supervisor.state = StateReady
	}
	return nil
}

func (supervisor *Supervisor) startOne(ctx context.Context, service Service) (stop StopFunc, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	return service.Start(ctx)
}

// Stop stops the started services in reverse order. Repeated calls share one
// shutdown and wait for it.
func (supervisor *Supervisor) Stop(ctx context.Context) {
	supervisor.mu.Lock()
	if supervisor.state == StateIdle || supervisor.state == StateStopped {
		supervisor.state = StateStopped
		if supervisor.stoppedAt == "" {
			supervisor.stoppedAt = supervisor.stamp()
		}
		supervisor.mu.Unlock()
		return
	}
	supervisor.mu.Unlock()
	supervisor.stopOnce.Do(func() {
		supervisor.mu.Lock()
		terminal := StateStopped
		if supervisor.state == StateFailed {
			terminal = StateFailed
		}
		supervisor.state = StateStopping
		supervisor.mu.Unlock()
		supervisor.stopStarted(ctx)
		supervisor.mu.Lock()
		supervisor.stoppedAt = supervisor.stamp()
		supervisor.state = terminal
		supervisor.mu.Unlock()
		close(supervisor.stopDone)
	})
	<-supervisor.stopDone
}

// ReportFailure records that a running service failed. A required failure
// moves the runtime to failed and runs OnRequiredFailure; an optional one
// degrades a ready runtime.
func (supervisor *Supervisor) ReportFailure(name string, cause error) error {
	if cause == nil {
		cause = errors.New("unknown failure")
	}
	supervisor.mu.Lock()
	var service *registered
	for _, candidate := range supervisor.services {
		if candidate.service.Name == name {
			service = candidate
			break
		}
	}
	if service == nil {
		supervisor.mu.Unlock()
		return fmt.Errorf("Unknown supervised service %q", name)
	}
	switch service.snapshot.State {
	case ComponentFailed, ComponentStopping, ComponentStopped:
		supervisor.mu.Unlock()
		return nil
	}
	service.snapshot.State = ComponentFailed
	service.snapshot.Error = cause.Error()
	if service.service.Optional {
		if supervisor.state == StateReady {
			supervisor.state = StateDegraded
		}
		supervisor.mu.Unlock()
		return nil
	}
	supervisor.state = StateFailed
	callback := supervisor.options.OnRequiredFailure
	supervisor.mu.Unlock()
	if callback != nil {
		go func() {
			defer func() { _ = recover() }()
			callback(name, cause)
		}()
	}
	return nil
}

// Snapshot answers the current state.
func (supervisor *Supervisor) Snapshot() Snapshot {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	components := Components{ByName: make(map[string]ComponentSnapshot, len(supervisor.services))}
	for _, service := range supervisor.services {
		components.Names = append(components.Names, service.service.Name)
		components.ByName[service.service.Name] = service.snapshot
	}
	return Snapshot{
		State:      supervisor.state,
		StartedAt:  supervisor.startedAt,
		ReadyAt:    supervisor.readyAt,
		StoppedAt:  supervisor.stoppedAt,
		Components: components,
	}
}

func (supervisor *Supervisor) stopStarted(ctx context.Context) {
	supervisor.mu.Lock()
	services := append([]*registered(nil), supervisor.services...)
	supervisor.mu.Unlock()
	for index := len(services) - 1; index >= 0; index-- {
		service := services[index]
		supervisor.mu.Lock()
		state := service.snapshot.State
		if service.stop == nil || (state != ComponentReady && state != ComponentFailed) {
			supervisor.mu.Unlock()
			continue
		}
		service.snapshot.State = ComponentStopping
		stop := service.stop
		supervisor.mu.Unlock()
		err := stopOne(ctx, stop)
		supervisor.mu.Lock()
		if err != nil {
			service.snapshot.State = ComponentFailed
			service.snapshot.Error = err.Error()
		} else {
			service.snapshot.State = ComponentStopped
			service.snapshot.StoppedAt = supervisor.stamp()
		}
		supervisor.mu.Unlock()
	}
}

func stopOne(ctx context.Context, stop StopFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	return stop(ctx)
}

func (supervisor *Supervisor) stamp() string {
	return Timestamp(supervisor.options.Now())
}

// Timestamp formats a time as JavaScript's Date.prototype.toISOString does.
func Timestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Marshal encodes JSON as JavaScript's JSON.stringify does: no HTML escaping
// and no trailing newline.
func Marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}
