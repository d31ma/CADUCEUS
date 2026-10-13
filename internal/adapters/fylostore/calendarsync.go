package fylostore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

func (store *AccountSync) CalendarState(ctx context.Context, scope accountsync.Scope) (calendarsync.State, error) {
	_, record, err := store.findCalendarState(ctx, scope)
	if errors.Is(err, accounts.ErrNotFound) {
		return calendarsync.State{}, nil
	}
	if err != nil {
		return calendarsync.State{}, err
	}
	calendars, err := store.providerCalendars(ctx, scope, record.ActiveGeneration)
	if err != nil {
		return calendarsync.State{}, err
	}
	resources, err := store.providerCalendarResources(ctx, scope, record.ActiveGeneration)
	if err != nil {
		return calendarsync.State{}, err
	}
	return calendarsync.State{PrincipalURL: record.PrincipalURL, HomeURL: record.HomeURL, Calendars: calendars, Resources: resources, LastSyncedAt: record.LastSyncedAt}, nil
}

// CommitCalendarSnapshot publishes a complete replacement through one active
// generation switch so readers never observe a partially synchronized set.
func (store *AccountSync) CommitCalendarSnapshot(ctx context.Context, lease accountsync.Lease, snapshot calendarsync.Snapshot) error {
	if err := calendarsync.ValidateSnapshot(lease.Scope, snapshot); err != nil {
		return err
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	_, syncState, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(syncState, lease, time.Now().UTC()) {
		return accountsync.ErrLeaseHeld
	}
	stateID, current, err := store.findCalendarState(ctx, lease.Scope)
	if err != nil && !errors.Is(err, accounts.ErrNotFound) {
		return err
	}
	generation, err := opaqueToken()
	if err != nil {
		return err
	}
	access := store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID)
	for _, calendar := range snapshot.Calendars {
		if _, err := store.documents.put(ctx, ProviderCalendars, newCalendarRecord(lease.Scope, generation, calendar), access); err != nil {
			return err
		}
	}
	for _, resource := range snapshot.Resources {
		if _, err := store.documents.put(ctx, ProviderCalendarResources, newCalendarResourceRecord(lease.Scope, generation, resource), access); err != nil {
			return err
		}
	}
	replacement := calendarStateRecord{AccountID: lease.Scope.AccountID, TenantID: lease.Scope.Owner.TenantID, PrincipalID: lease.Scope.Owner.PrincipalID, OwnerAddress: lease.Scope.Owner.Address,
		PrincipalURL: snapshot.PrincipalURL, HomeURL: snapshot.HomeURL, ActiveGeneration: generation, LastSyncedAt: timePointer(snapshot.SyncedAt.UTC())}
	if stateID == "" {
		if _, err := store.documents.put(ctx, ProviderCalendarState, replacement, access); err != nil {
			return err
		}
	} else if err := store.documents.patch(ctx, ProviderCalendarState, stateID, replacement, current, access); err != nil {
		return err
	}
	if err := store.deleteInactiveCalendarGeneration(ctx, ProviderCalendars, lease.Scope, generation); err != nil {
		return err
	}
	return store.deleteInactiveCalendarGeneration(ctx, ProviderCalendarResources, lease.Scope, generation)
}

func (store *AccountSync) ListProviderCalendarResources(ctx context.Context, owner accounts.Owner) ([]calendarsync.OwnedResource, error) {
	stateDocuments, err := store.documents.find(ctx, ProviderCalendarState, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	type activeGeneration struct {
		generation string
		address    string
	}
	active := make(map[accounts.AccountID]activeGeneration)
	for _, raw := range stateDocuments {
		var state calendarStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, fmt.Errorf("decode FYLO provider calendar state: %w", err)
		}
		if state.TenantID == owner.TenantID && state.PrincipalID == owner.PrincipalID {
			active[state.AccountID] = activeGeneration{generation: state.ActiveGeneration, address: state.OwnerAddress}
		}
	}
	calendarDocuments, err := store.documents.find(ctx, ProviderCalendars, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	calendars := make(map[string]calendarsync.Calendar)
	for _, raw := range calendarDocuments {
		var record calendarRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider calendar: %w", err)
		}
		if current, ok := active[record.AccountID]; ok && current.generation == record.Generation && record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID {
			calendars[string(record.AccountID)+"\x00"+record.StableID] = record.Calendar
		}
	}
	resourceDocuments, err := store.documents.find(ctx, ProviderCalendarResources, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	result := make([]calendarsync.OwnedResource, 0)
	for _, raw := range resourceDocuments {
		var record calendarResourceRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider calendar resource: %w", err)
		}
		current, ok := active[record.AccountID]
		calendar, calendarOK := calendars[string(record.AccountID)+"\x00"+record.CalendarID]
		if !ok || !calendarOK || current.generation != record.Generation || record.TenantID != owner.TenantID || record.PrincipalID != owner.PrincipalID {
			continue
		}
		result = append(result, calendarsync.OwnedResource{OwnerAddress: current.address, CalendarName: calendar.Name, CalendarColor: calendar.Color, Resource: record.Resource})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

func (store *AccountSync) ListProviderCalendars(ctx context.Context, owner accounts.Owner) ([]calendarsync.OwnedCalendar, error) {
	stateDocuments, err := store.documents.find(ctx, ProviderCalendarState, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	active := make(map[accounts.AccountID]string)
	for _, raw := range stateDocuments {
		var state calendarStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, fmt.Errorf("decode FYLO provider calendar state: %w", err)
		}
		if state.TenantID == owner.TenantID && state.PrincipalID == owner.PrincipalID {
			active[state.AccountID] = state.ActiveGeneration
		}
	}
	documents, err := store.documents.find(ctx, ProviderCalendars, store.documents.readAccess(owner))
	if err != nil {
		return nil, err
	}
	result := make([]calendarsync.OwnedCalendar, 0)
	for _, raw := range documents {
		var record calendarRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode FYLO provider calendar: %w", err)
		}
		if generation, ok := active[record.AccountID]; ok && generation == record.Generation && record.TenantID == owner.TenantID && record.PrincipalID == owner.PrincipalID {
			result = append(result, calendarsync.OwnedCalendar{AccountID: record.AccountID, Calendar: record.Calendar})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].AccountID == result[right].AccountID {
			if result[left].Name == result[right].Name {
				return result[left].StableID < result[right].StableID
			}
			return result[left].Name < result[right].Name
		}
		return result[left].AccountID < result[right].AccountID
	})
	return result, nil
}

func (store *AccountSync) ProviderCalendarResource(ctx context.Context, scope accountsync.Scope, resourceID string) (calendarsync.Resource, error) {
	_, state, err := store.findCalendarState(ctx, scope)
	if err != nil {
		return calendarsync.Resource{}, err
	}
	resources, err := store.providerCalendarResources(ctx, scope, state.ActiveGeneration)
	if err != nil {
		return calendarsync.Resource{}, err
	}
	for _, resource := range resources {
		if resource.ID == resourceID {
			return resource, nil
		}
	}
	return calendarsync.Resource{}, calendarsync.ErrResourceNotFound
}

func (store *AccountSync) EnqueueCalendarMutation(ctx context.Context, scope accountsync.Scope, mutation calendarsync.Mutation) (calendarsync.Mutation, error) {
	if err := calendarsync.ValidateMutation(mutation.MutationRequest); err != nil {
		return calendarsync.Mutation{}, err
	}
	if mutation.Kind == calendarsync.MutationCreate && mutation.Desired.AccountID != scope.AccountID {
		return calendarsync.Mutation{}, calendarsync.ErrInvalidMutation
	}
	store.mutation.Lock()
	defer store.mutation.Unlock()
	documents, err := store.documents.find(ctx, ProviderCalendarMutations, store.documents.readAccess(scope.Owner))
	if err != nil {
		return calendarsync.Mutation{}, err
	}
	for _, raw := range documents {
		var record calendarMutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return calendarsync.Mutation{}, err
		}
		if record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID && record.IdempotencyKey == mutation.IdempotencyKey {
			record.Mutation.Scope = scope
			storedRequest, candidateRequest := record.Mutation.MutationRequest, mutation.MutationRequest
			storedRequest.ID, candidateRequest.ID = "", ""
			if reflect.DeepEqual(storedRequest, candidateRequest) {
				return record.Mutation, nil
			}
			return calendarsync.Mutation{}, accounts.ErrConflict
		}
	}
	mutation.Scope = scope
	if mutation.State == "" {
		mutation.State = calendarsync.MutationQueued
	}
	if _, err := store.documents.put(ctx, ProviderCalendarMutations, newCalendarMutationRecord(mutation), store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
		return calendarsync.Mutation{}, err
	}
	return mutation, nil
}

func (store *AccountSync) PendingCalendarMutations(ctx context.Context, lease accountsync.Lease, limit int) ([]calendarsync.Mutation, error) {
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return nil, err
	}
	if !validLease(state, lease, time.Now().UTC()) {
		return nil, accountsync.ErrLeaseHeld
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	documents, err := store.documents.find(ctx, ProviderCalendarMutations, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []calendarsync.Mutation{}
	for _, raw := range documents {
		var record calendarMutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		if record.AccountID == lease.Scope.AccountID && record.TenantID == lease.Scope.Owner.TenantID && record.PrincipalID == lease.Scope.Owner.PrincipalID && (record.State == calendarsync.MutationQueued || record.State == calendarsync.MutationApplying) {
			record.Mutation.Scope = lease.Scope
			result = append(result, record.Mutation)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].CreatedAt.Before(result[right].CreatedAt) })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (store *AccountSync) UpdateCalendarMutation(ctx context.Context, lease accountsync.Lease, replacement calendarsync.Mutation) error {
	_, state, err := store.findState(ctx, lease.Scope)
	if err != nil {
		return err
	}
	if !validLease(state, lease, time.Now().UTC()) || replacement.Scope != lease.Scope {
		return accountsync.ErrLeaseHeld
	}
	documents, err := store.documents.find(ctx, ProviderCalendarMutations, store.documents.readAccess(lease.Scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var current calendarMutationRecord
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.ID != replacement.ID || current.AccountID != lease.Scope.AccountID || current.TenantID != lease.Scope.Owner.TenantID || current.PrincipalID != lease.Scope.Owner.PrincipalID {
			continue
		}
		if !validCalendarMutationTransition(current.State, replacement.State) {
			return calendarsync.ErrInvalidMutation
		}
		return store.documents.patch(ctx, ProviderCalendarMutations, id, newCalendarMutationRecord(replacement), current, store.documents.writeAccess(lease.Scope.Owner, lease.Scope.AccountID))
	}
	return calendarsync.ErrResourceNotFound
}

func (store *AccountSync) CalendarMutation(ctx context.Context, scope accountsync.Scope, mutationID string) (calendarsync.Mutation, error) {
	documents, err := store.documents.find(ctx, ProviderCalendarMutations, store.documents.readAccess(scope.Owner))
	if err != nil {
		return calendarsync.Mutation{}, err
	}
	for _, raw := range documents {
		var record calendarMutationRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return calendarsync.Mutation{}, err
		}
		if record.ID == mutationID && record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID {
			record.Mutation.Scope = scope
			return record.Mutation, nil
		}
	}
	return calendarsync.Mutation{}, calendarsync.ErrResourceNotFound
}

func (store *AccountSync) findCalendarState(ctx context.Context, scope accountsync.Scope) (string, calendarStateRecord, error) {
	documents, err := store.documents.find(ctx, ProviderCalendarState, store.documents.readAccess(scope.Owner))
	if err != nil {
		return "", calendarStateRecord{}, err
	}
	for id, raw := range documents {
		var state calendarStateRecord
		if err := json.Unmarshal(raw, &state); err != nil {
			return "", calendarStateRecord{}, err
		}
		if state.AccountID == scope.AccountID && state.TenantID == scope.Owner.TenantID && state.PrincipalID == scope.Owner.PrincipalID {
			return id, state, nil
		}
	}
	return "", calendarStateRecord{}, accounts.ErrNotFound
}

func (store *AccountSync) providerCalendars(ctx context.Context, scope accountsync.Scope, generation string) ([]calendarsync.Calendar, error) {
	documents, err := store.documents.find(ctx, ProviderCalendars, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []calendarsync.Calendar{}
	for _, raw := range documents {
		var record calendarRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		if record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID && record.Generation == generation {
			result = append(result, record.Calendar)
		}
	}
	return result, nil
}

func (store *AccountSync) providerCalendarResources(ctx context.Context, scope accountsync.Scope, generation string) ([]calendarsync.Resource, error) {
	documents, err := store.documents.find(ctx, ProviderCalendarResources, store.documents.readAccess(scope.Owner))
	if err != nil {
		return nil, err
	}
	result := []calendarsync.Resource{}
	for _, raw := range documents {
		var record calendarResourceRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		if record.AccountID == scope.AccountID && record.TenantID == scope.Owner.TenantID && record.PrincipalID == scope.Owner.PrincipalID && record.Generation == generation {
			result = append(result, record.Resource)
		}
	}
	return result, nil
}

func (store *AccountSync) deleteInactiveCalendarGeneration(ctx context.Context, collection string, scope accountsync.Scope, active string) error {
	documents, err := store.documents.find(ctx, collection, store.documents.readAccess(scope.Owner))
	if err != nil {
		return err
	}
	for id, raw := range documents {
		var identity struct {
			AccountID   accounts.AccountID `json:"accountId"`
			TenantID    string             `json:"tenantId"`
			PrincipalID string             `json:"principalId"`
			Generation  string             `json:"generation"`
		}
		if err := json.Unmarshal(raw, &identity); err != nil {
			return err
		}
		if identity.AccountID == scope.AccountID && identity.TenantID == scope.Owner.TenantID && identity.PrincipalID == scope.Owner.PrincipalID && identity.Generation != active {
			if err := store.documents.delete(ctx, collection, id, store.documents.writeAccess(scope.Owner, scope.AccountID)); err != nil {
				return err
			}
		}
	}
	return nil
}

type calendarStateRecord struct {
	AccountID        accounts.AccountID `json:"accountId"`
	TenantID         string             `json:"tenantId"`
	PrincipalID      string             `json:"principalId"`
	OwnerAddress     string             `json:"ownerAddress"`
	PrincipalURL     string             `json:"principalUrl"`
	HomeURL          string             `json:"homeUrl"`
	ActiveGeneration string             `json:"activeGeneration"`
	LastSyncedAt     *time.Time         `json:"lastSyncedAt,omitempty"`
}

type calendarRecord struct {
	calendarsync.Calendar
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
	Generation   string             `json:"generation"`
}

type calendarResourceRecord struct {
	calendarsync.Resource
	TenantID     string `json:"tenantId"`
	PrincipalID  string `json:"principalId"`
	OwnerAddress string `json:"ownerAddress"`
	Generation   string `json:"generation"`
}

type calendarMutationRecord struct {
	calendarsync.Mutation
	AccountID    accounts.AccountID `json:"accountId"`
	TenantID     string             `json:"tenantId"`
	PrincipalID  string             `json:"principalId"`
	OwnerAddress string             `json:"ownerAddress"`
}

func newCalendarRecord(scope accountsync.Scope, generation string, calendar calendarsync.Calendar) calendarRecord {
	return calendarRecord{Calendar: calendar, AccountID: scope.AccountID, TenantID: scope.Owner.TenantID, PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address, Generation: generation}
}

func newCalendarResourceRecord(scope accountsync.Scope, generation string, resource calendarsync.Resource) calendarResourceRecord {
	return calendarResourceRecord{Resource: resource, TenantID: scope.Owner.TenantID, PrincipalID: scope.Owner.PrincipalID, OwnerAddress: scope.Owner.Address, Generation: generation}
}

func newCalendarMutationRecord(mutation calendarsync.Mutation) calendarMutationRecord {
	return calendarMutationRecord{Mutation: mutation, AccountID: mutation.Scope.AccountID, TenantID: mutation.Scope.Owner.TenantID, PrincipalID: mutation.Scope.Owner.PrincipalID, OwnerAddress: mutation.Scope.Owner.Address}
}

func validCalendarMutationTransition(current, next calendarsync.MutationState) bool {
	allowed := map[calendarsync.MutationState]map[calendarsync.MutationState]bool{
		calendarsync.MutationQueued:   {calendarsync.MutationApplying: true},
		calendarsync.MutationApplying: {calendarsync.MutationApplying: true, calendarsync.MutationApplied: true, calendarsync.MutationConflict: true, calendarsync.MutationFailed: true},
	}
	return allowed[current][next]
}
