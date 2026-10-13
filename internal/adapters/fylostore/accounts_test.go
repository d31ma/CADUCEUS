package fylostore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/platform/fylo"
)

func TestAccountsRepositoryPersistsSeparatedRecordsWithPrincipalAndAccountAccess(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	account := exampleAccount()
	secret := accounts.EncryptedSecret{KeyVersion: 7, Nonce: "opaque-nonce", Ciphertext: "opaque-ciphertext"}

	created, err := repository.Create(context.Background(), account, secret)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != account.ID || created.Revision != 1 {
		t.Fatalf("Create() = %#v", created)
	}
	if len(machine.puts) != 3 || machine.puts[0].collection != ProviderCredentials || machine.puts[1].collection != ProviderAccounts || machine.puts[2].collection != ProviderSchedule {
		t.Fatalf("put sequence = %#v", machine.puts)
	}
	wantUID := virtualID("sesame-principal", account.Owner.TenantID+"\x00"+account.Owner.PrincipalID)
	wantGID := virtualID("provider-account", account.Owner.TenantID+"\x00"+string(account.ID))
	for _, put := range machine.puts[:2] {
		if put.access.uid != wantUID || put.access.gid != wantGID || put.access.mode != 0o660 {
			t.Fatalf("%s access = %#v, want uid=%d gid=%d mode=0660", put.collection, put.access, wantUID, wantGID)
		}
	}
	if machine.puts[2].access.uid != schedulerUID() || machine.puts[2].access.mode != 0o600 {
		t.Fatalf("schedule access = %#v", machine.puts[2].access)
	}
	publicJSON, _ := json.Marshal(machine.puts[1].data)
	if contains(string(publicJSON), "opaque-ciphertext") || contains(string(publicJSON), "opaque-nonce") {
		t.Fatalf("public record contains secret envelope: %s", publicJSON)
	}
	secretJSON, _ := json.Marshal(machine.puts[0].data)
	if contains(string(secretJSON), account.MailboxAddress) {
		t.Fatalf("credential record contains mailbox presentation data: %s", secretJSON)
	}
}

func TestAccountsRepositoryScopesGetAndListToExactOwner(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	account := exampleAccount()
	if _, err := repository.Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "n", Ciphertext: "c"}); err != nil {
		t.Fatal(err)
	}

	found, err := repository.Get(context.Background(), account.Owner, account.ID)
	if err != nil || found.MailboxAddress != account.MailboxAddress {
		t.Fatalf("Get() = %#v, %v", found, err)
	}
	foreign := accounts.Owner{TenantID: account.Owner.TenantID, PrincipalID: "principal-bob", Address: "bob@example.com"}
	if _, err := repository.Get(context.Background(), foreign, account.ID); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign Get() error = %v, want ErrNotFound", err)
	}
	listed, err := repository.List(context.Background(), foreign)
	if err != nil || len(listed) != 0 {
		t.Fatalf("foreign List() = %#v, %v", listed, err)
	}
}

func TestAccountsRepositoryLoadsCredentialEnvelopeOnlyForExactOwnerAndProvider(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	account := exampleAccount()
	want := accounts.EncryptedSecret{KeyVersion: 4, Nonce: "opaque-nonce-value", Ciphertext: "opaque-ciphertext-value"}
	if _, err := repository.Create(context.Background(), account, want); err != nil {
		t.Fatal(err)
	}

	got, err := repository.GetSecret(context.Background(), account.Owner, account.ID, account.Provider)
	if err != nil || got != want {
		t.Fatalf("GetSecret() = %#v, %v", got, err)
	}
	foreign := accounts.Owner{TenantID: account.Owner.TenantID, PrincipalID: "principal-bob", Address: "bob@example.com"}
	if _, err := repository.GetSecret(context.Background(), foreign, account.ID, account.Provider); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign GetSecret() error = %v, want ErrNotFound", err)
	}
	if _, err := repository.GetSecret(context.Background(), account.Owner, account.ID, accounts.ProviderGmail); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("wrong-provider GetSecret() error = %v, want ErrNotFound", err)
	}
}

func TestAccountsRepositoryEnforcesRevisionOnUpdateAndDeleteMarker(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	account := exampleAccount()
	if _, err := repository.Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "n", Ciphertext: "c"}); err != nil {
		t.Fatal(err)
	}

	account.State = accounts.StateSyncing
	account.Revision = 2
	account.UpdatedAt = account.UpdatedAt.Add(time.Minute)
	updated, err := repository.Update(context.Background(), account, 1)
	if err != nil || updated.State != accounts.StateSyncing {
		t.Fatalf("Update() = %#v, %v", updated, err)
	}
	if _, err := repository.Update(context.Background(), account, 1); !errors.Is(err, accounts.ErrConflict) {
		t.Fatalf("stale Update() error = %v, want ErrConflict", err)
	}
	account.State = accounts.StateDeleting
	account.Revision = 3
	markedAt := time.Now().UTC()
	if err := repository.MarkDeleting(context.Background(), account, 2); err != nil {
		t.Fatalf("MarkDeleting() error = %v", err)
	}
	found, err := repository.Get(context.Background(), account.Owner, account.ID)
	if err != nil || found.State != accounts.StateDeleting || found.Revision != 3 {
		t.Fatalf("deleted marker = %#v, %v", found, err)
	}
	if _, err := repository.GetSecret(context.Background(), account.Owner, account.ID, account.Provider); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("deleted account credential error = %v, want ErrNotFound", err)
	}
	_, schedule, err := repository.findSchedule(context.Background(), account.Owner, account.ID)
	if err != nil || schedule.NextRunAt.Before(markedAt) || schedule.NextRunAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("deletion schedule = %#v, %v", schedule, err)
	}
	if err := repository.FinalizeDelete(context.Background(), account.Owner, account.ID); err != nil {
		t.Fatalf("FinalizeDelete() error = %v", err)
	}
	if _, err := repository.Get(context.Background(), account.Owner, account.ID); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("finalized account error = %v, want ErrNotFound", err)
	}
	if _, _, err := repository.findSchedule(context.Background(), account.Owner, account.ID); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("finalized schedule error = %v, want ErrNotFound", err)
	}
}

func TestAccountsRepositoryReplacesCredentialAndProjectionTogether(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	account := exampleAccount()
	if _, err := repository.Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "old-nonce", Ciphertext: "old-cipher"}); err != nil {
		t.Fatal(err)
	}
	account.State = accounts.StateReady
	account.Revision = 2
	account.Capabilities.MailWrite = true
	replacement := accounts.EncryptedSecret{KeyVersion: 2, Nonce: "new-nonce", Ciphertext: "new-cipher"}
	updated, err := repository.ReplaceCredential(context.Background(), account, replacement, 1)
	if err != nil || updated.Revision != 2 || !updated.Capabilities.MailWrite {
		t.Fatalf("ReplaceCredential() = %#v, %v", updated, err)
	}
	stored, err := repository.GetSecret(context.Background(), account.Owner, account.ID, account.Provider)
	if err != nil || stored != replacement {
		t.Fatalf("replacement credential = %#v, %v", stored, err)
	}
}

func TestAccountsRepositoryTakesOAuthTransactionOnceForExactOwner(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	owner := exampleAccount().Owner
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	transaction := accounts.OAuthTransaction{
		StateHash: strings.Repeat("a", 64), Owner: owner, Provider: accounts.ProviderMicrosoft,
		Request:    accounts.OAuthConnectionRequest{Provider: accounts.ProviderMicrosoft, MailboxAddress: "shared@example.com", AuthIdentity: "delegate@example.com", SharedMailboxAddress: "shared@example.com", Label: "Support", Color: "#336699"},
		Verifier:   accounts.EncryptedSecret{KeyVersion: 1, Nonce: "opaque-nonce-value", Ciphertext: "opaque-ciphertext-value"},
		VerifierID: "oauth_AAAAAAAAAAAAAAAAAAAAAAAA", CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	if err := repository.Put(context.Background(), transaction); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	foreign := owner
	foreign.PrincipalID = "principal-bob"
	if _, err := repository.Take(context.Background(), foreign, accounts.ProviderMicrosoft, transaction.StateHash, now); !errors.Is(err, accounts.ErrOAuthTransaction) {
		t.Fatalf("foreign Take() error = %v", err)
	}
	taken, err := repository.Take(context.Background(), owner, accounts.ProviderMicrosoft, transaction.StateHash, now)
	if err != nil || taken.Request.SharedMailboxAddress != "shared@example.com" || taken.Verifier.Ciphertext != transaction.Verifier.Ciphertext {
		t.Fatalf("Take() = %#v, %v", taken, err)
	}
	if _, err := repository.Take(context.Background(), owner, accounts.ProviderMicrosoft, transaction.StateHash, now); !errors.Is(err, accounts.ErrOAuthTransaction) {
		t.Fatalf("replayed Take() error = %v", err)
	}
}

func TestAccountsRepositoryReplacesAnOwnersPendingOAuthTransaction(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	owner := exampleAccount().Owner
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	transaction := accounts.OAuthTransaction{
		StateHash: strings.Repeat("a", 64), Owner: owner, Provider: accounts.ProviderMicrosoft,
		Request:    accounts.OAuthConnectionRequest{Provider: accounts.ProviderMicrosoft, MailboxAddress: "owner@example.com", AuthIdentity: "owner@example.com", Label: "Microsoft", Color: "#336699"},
		Verifier:   accounts.EncryptedSecret{KeyVersion: 1, Nonce: "opaque-nonce-value", Ciphertext: "opaque-ciphertext-value"},
		VerifierID: "oauth_AAAAAAAAAAAAAAAAAAAAAAAA", CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	if err := repository.Put(context.Background(), transaction); err != nil {
		t.Fatal(err)
	}
	second := transaction
	second.StateHash = strings.Repeat("b", 64)
	second.VerifierID = "oauth_BBBBBBBBBBBBBBBBBBBBBBBB"
	if err := repository.Put(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Take(context.Background(), owner, accounts.ProviderMicrosoft, transaction.StateHash, now); !errors.Is(err, accounts.ErrOAuthTransaction) {
		t.Fatalf("superseded Take() error = %v", err)
	}
	if _, err := repository.Take(context.Background(), owner, accounts.ProviderMicrosoft, second.StateHash, now); err != nil {
		t.Fatalf("latest Take() error = %v", err)
	}
}

func TestAccountsRepositoryRotatesCredentialEnvelopeWithoutPublicRevision(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine)
	account := exampleAccount()
	old := accounts.EncryptedSecret{KeyVersion: 1, Nonce: "old-nonce-value", Ciphertext: "old-cipher-value"}
	if _, err := repository.Create(context.Background(), account, old); err != nil {
		t.Fatal(err)
	}
	replacement := accounts.EncryptedSecret{KeyVersion: 2, Nonce: "new-nonce-value", Ciphertext: "new-cipher-value"}
	if err := repository.RotateSecret(context.Background(), account, old, replacement); err != nil {
		t.Fatalf("RotateSecret() error = %v", err)
	}
	stored, err := repository.GetSecret(context.Background(), account.Owner, account.ID, account.Provider)
	if err != nil || stored != replacement {
		t.Fatalf("rotated secret = %#v, %v", stored, err)
	}
	if err := repository.RotateSecret(context.Background(), account, old, replacement); !errors.Is(err, accounts.ErrConflict) {
		t.Fatalf("stale RotateSecret() error = %v", err)
	}
	public, err := repository.Get(context.Background(), account.Owner, account.ID)
	if err != nil || public.Revision != account.Revision {
		t.Fatalf("public account changed during token rotation: %#v, %v", public, err)
	}
}

func TestAccountsRepositoryOmitsMachineOwnershipWhenExplicitlyDisabled(t *testing.T) {
	machine := newMemoryCaller()
	repository := NewAccounts(machine, false)
	account := exampleAccount()
	if _, err := repository.Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "nonce", Ciphertext: "cipher"}); err != nil {
		t.Fatal(err)
	}
	for _, put := range machine.puts[:2] {
		if put.hasAccess {
			t.Fatalf("%s unexpectedly carried machine access: %#v", put.collection, put.access)
		}
	}
	if machine.puts[2].hasAccess {
		t.Fatalf("scheduler index unexpectedly carried machine access in fallback mode: %#v", machine.puts[2])
	}
	foreign := account.Owner
	foreign.PrincipalID = "principal-bob"
	if listed, err := repository.List(context.Background(), foreign); err != nil || len(listed) != 0 {
		t.Fatalf("application owner filter failed without POSIX mode: %#v, %v", listed, err)
	}
}

func TestAccountsRepositoryCompensatesCredentialWriteWhenAccountWriteFails(t *testing.T) {
	machine := newMemoryCaller()
	machine.failPutCollection = ProviderAccounts
	repository := NewAccounts(machine)
	_, err := repository.Create(context.Background(), exampleAccount(), accounts.EncryptedSecret{KeyVersion: 1, Nonce: "n", Ciphertext: "c"})
	if err == nil {
		t.Fatal("Create() error = nil, want failure")
	}
	if machine.deletes != 1 || len(machine.documents) != 0 {
		t.Fatalf("compensation deletes=%d documents=%#v", machine.deletes, machine.documents)
	}
}

func TestEnsureProviderCollectionsCreatesOnlyGoOwnedCollections(t *testing.T) {
	caller := &collectionCaller{}
	if err := EnsureProviderCollections(context.Background(), caller); err != nil {
		t.Fatalf("EnsureProviderCollections() error = %v", err)
	}
	want := []string{ProviderAccounts, ProviderCredentials, ProviderOAuthTransactions, ProviderSyncState, ProviderFolders, ProviderMessages, ProviderNotifications, ProviderMutations, ProviderSubmissions, ProviderSchedule, ProviderContactState, ProviderContactBooks, ProviderContacts, ProviderContactMutations, ProviderCalendarState, ProviderCalendars, ProviderCalendarResources, ProviderCalendarMutations, ProviderRawFiles, ProviderSubmitFiles}
	if fmt.Sprint(caller.created) != fmt.Sprint(want) {
		t.Fatalf("created = %#v, want %#v", caller.created, want)
	}
}

func exampleAccount() accounts.ExternalAccount {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	return accounts.ExternalAccount{
		ID:       "ext_AAAAAAAAAAAAAAAAAAAA",
		Owner:    accounts.Owner{TenantID: "tenant-1", PrincipalID: "principal-alice", Address: "alice@example.com"},
		Provider: accounts.ProviderIMAP, MailboxAddress: "alice@remote.example", Label: "Work", Color: "#336699",
		State: accounts.StateReady, Capabilities: accounts.Capabilities{MailRead: true, Send: true}, Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
}

type accessCall struct {
	uid  int
	gid  int
	mode int
}

type putCall struct {
	collection string
	data       map[string]any
	access     accessCall
	hasAccess  bool
}

type storedDocument struct {
	collection string
	data       map[string]any
	uid        int
	file       []byte
}

type memoryCaller struct {
	documents         map[string]storedDocument
	puts              []putCall
	next              int
	deletes           int
	failPutCollection string
}

func newMemoryCaller() *memoryCaller {
	return &memoryCaller{documents: map[string]storedDocument{}}
}

func (caller *memoryCaller) Call(_ context.Context, request map[string]any) (fylo.Response, error) {
	switch request["op"] {
	case "putData":
		return caller.put(request)
	case "findDocs":
		return caller.find(request)
	case "patchDoc":
		return caller.patch(request)
	case "delDoc":
		return caller.delete(request)
	case "getFileData":
		return caller.getFile(request)
	default:
		return fylo.Response{}, fmt.Errorf("unexpected op %v", request["op"])
	}
}

func (caller *memoryCaller) put(request map[string]any) (fylo.Response, error) {
	collection := request["collection"].(string)
	if collection == caller.failPutCollection {
		return fylo.Response{}, errors.New("injected put failure")
	}
	caller.next++
	id := fmt.Sprintf("doc-%d", caller.next)
	data := cloneMap(request["data"])
	var content []byte
	if file, ok := request["file"].(map[string]any); ok {
		content, _ = os.ReadFile(file["path"].(string))
	}
	accessValue, hasAccess := request["access"]
	access := decodeAccess(accessValue)
	caller.puts = append(caller.puts, putCall{collection: collection, data: data, access: access, hasAccess: hasAccess})
	caller.documents[id] = storedDocument{collection: collection, data: data, uid: access.uid, file: content}
	return success(id), nil
}

func (caller *memoryCaller) find(request map[string]any) (fylo.Response, error) {
	collection := request["collection"].(string)
	access := decodeAccess(request["access"])
	result := map[string]map[string]any{}
	for id, document := range caller.documents {
		if document.collection == collection && document.uid == access.uid {
			result[id] = cloneMap(document.data)
		}
	}
	return success(result), nil
}

func (caller *memoryCaller) patch(request map[string]any) (fylo.Response, error) {
	id := request["id"].(string)
	document, found := caller.documents[id]
	if !found {
		return fylo.Response{}, errors.New("missing patch target")
	}
	document.data = cloneMap(request["newDoc"])
	caller.documents[id] = document
	return success(id), nil
}

func (caller *memoryCaller) delete(request map[string]any) (fylo.Response, error) {
	delete(caller.documents, request["id"].(string))
	caller.deletes++
	return success(true), nil
}

func (caller *memoryCaller) getFile(request map[string]any) (fylo.Response, error) {
	document, found := caller.documents[request["id"].(string)]
	if !found || document.collection != request["collection"] {
		return success(map[string]any{}), nil
	}
	return success(map[string]any{"id": request["id"], "contentLength": len(document.file), "encoding": "base64", "data": base64.StdEncoding.EncodeToString(document.file)}), nil
}

func decodeAccess(value any) accessCall {
	input, ok := value.(map[string]any)
	if !ok {
		return accessCall{}
	}
	return accessCall{uid: input["uid"].(int), gid: integer(input["gid"]), mode: integer(input["mode"])}
}

func integer(value any) int {
	if value == nil {
		return 0
	}
	return value.(int)
}

func cloneMap(value any) map[string]any {
	encoded, _ := json.Marshal(value)
	result := map[string]any{}
	_ = json.Unmarshal(encoded, &result)
	return result
}

func success(result any) fylo.Response {
	encoded, _ := json.Marshal(result)
	return fylo.Response{OK: true, Result: encoded}
}

type collectionCaller struct {
	created []string
}

func (caller *collectionCaller) Call(_ context.Context, request map[string]any) (fylo.Response, error) {
	wantKind := "document"
	if request["collection"] == ProviderRawFiles || request["collection"] == ProviderSubmitFiles {
		wantKind = "file"
	}
	if request["op"] != "createCollection" || request["kind"] != wantKind {
		return fylo.Response{}, fmt.Errorf("unexpected collection request %#v", request)
	}
	caller.created = append(caller.created, request["collection"].(string))
	return success(true), nil
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
