package fylostore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestAccountSyncPersistsLeaseIndexesRawBucketAndCheckpointInOrder(t *testing.T) {
	machine := newMemoryCaller()
	store, err := NewAccountSync(machine, true, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := syncScope()
	now := time.Now().UTC()
	lease, err := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AcquireLease(context.Background(), scope, "worker-b", now, time.Minute); !errors.Is(err, accountsync.ErrLeaseHeld) {
		t.Fatalf("contending lease error = %v", err)
	}
	raw := []byte("Subject: imported\r\n\r\nbody")
	checkpoint, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{
		CursorAfter: "uidvalidity=4;uid=9", Phase: accounts.PhaseHeaders, Completed: 1,
		Folders:  []accountsync.Folder{{StableID: "INBOX", Revision: "4", Name: "INBOX", Role: "inbox", Attributes: []string{}}},
		Messages: []accountsync.Message{{StableID: "INBOX:9", Revision: "9", FolderStableID: "INBOX", Subject: "imported", ReceivedAt: now, Flags: []string{}, Size: uint64(len(raw)), RawRFC822: raw}},
	})
	if err != nil || checkpoint.Cursor != "uidvalidity=4;uid=9" || checkpoint.Version != 1 {
		t.Fatalf("CommitBatch() = %#v, %v", checkpoint, err)
	}
	for collection, want := range map[string]int{ProviderSyncState: 1, ProviderFolders: 1, ProviderMessages: 1, ProviderRawFiles: 1} {
		if got := countCollection(machine, collection); got != want {
			t.Fatalf("%s count=%d want=%d", collection, got, want)
		}
	}
	message := oneDocument(machine, ProviderMessages)
	if message["rawBucketId"] == "" || message["accountId"] != string(scope.AccountID) || message["tenantId"] != scope.Owner.TenantID {
		t.Fatalf("provider message = %#v", message)
	}
	if _, found := message["rawRFC822"]; found {
		t.Fatal("raw RFC 822 bytes leaked into document")
	}
	if err := store.ReleaseLease(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	reacquired, err := store.AcquireLease(context.Background(), scope, "worker-b", now.Add(time.Second), time.Minute)
	if err != nil || reacquired.Holder != "worker-b" {
		t.Fatalf("reacquired lease = %#v, %v", reacquired, err)
	}
}

func TestAccountSyncReschedulePreservesConcurrentRetryWake(t *testing.T) {
	machine := newMemoryCaller()
	account := exampleAccount()
	account.CreatedAt = time.Now().UTC().Add(-time.Minute)
	if _, err := NewAccounts(machine).Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "n", Ciphertext: "c"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewAccountSync(machine, true, t.TempDir())
	due, err := store.DueAccounts(context.Background(), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC), 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("DueAccounts() = %#v, %v schedule=%#v count=%d", due, err, oneDocument(machine, ProviderSchedule), countCollection(machine, ProviderSchedule))
	}
	watches, err := store.WatchAccounts(context.Background(), 10)
	if err != nil || len(watches) != 1 || watches[0].Scope != due[0].Scope || watches[0].Provider != account.Provider {
		t.Fatalf("WatchAccounts() = %#v, %v", watches, err)
	}
	stale := due[0]
	if err := store.Retry(context.Background(), stale.Scope); err != nil {
		t.Fatal(err)
	}
	stale.NextRunAt = time.Now().UTC().Add(time.Minute)
	if err := store.Reschedule(context.Background(), stale); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := store.Diagnostics(context.Background(), stale.Scope)
	if err != nil || diagnostics.NextRunAt.After(time.Now().UTC().Add(5*time.Second)) {
		t.Fatalf("concurrent wake was overwritten: %#v, %v", diagnostics, err)
	}
}

func TestAccountSyncRetryReplacesStableMessageWithoutDuplicateAndIsolatesOwner(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	first := []byte("Subject: one\r\n\r\none")
	_, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{
		CursorAfter: "1", Phase: accounts.PhaseBodies,
		Messages: []accountsync.Message{{StableID: "INBOX:1", Revision: "1", FolderStableID: "INBOX", Subject: "one", ReceivedAt: now, Flags: []string{}, Size: uint64(len(first)), RawRFC822: first}},
	})
	if err != nil {
		t.Fatal(err)
	}
	second := []byte("Subject: two\r\n\r\ntwo")
	_, err = store.CommitBatch(context.Background(), lease, 1, accountsync.Batch{
		CursorBefore: "1", CursorAfter: "2", Phase: accounts.PhaseBodies,
		Messages: []accountsync.Message{{StableID: "INBOX:1", Revision: "2", FolderStableID: "INBOX", Subject: "two", ReceivedAt: now, Flags: []string{}, Size: uint64(len(second)), RawRFC822: second}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := countCollection(machine, ProviderMessages); got != 1 {
		t.Fatalf("message count = %d", got)
	}
	if got := countCollection(machine, ProviderRawFiles); got != 1 {
		t.Fatalf("raw bucket count = %d", got)
	}
	if subject := oneDocument(machine, ProviderMessages)["subject"]; subject != "two" {
		t.Fatalf("subject = %v", subject)
	}

	foreign := scope
	foreign.Owner.PrincipalID = "bob"
	foreign.Owner.Address = "bob@example.com"
	checkpoint, err := store.Checkpoint(context.Background(), foreign)
	if err != nil || checkpoint.Version != 0 {
		t.Fatalf("foreign checkpoint = %#v, %v", checkpoint, err)
	}
}

func TestAccountSyncUnifiedIndexFiltersOwnerFolderFlagsSearchAndReadsRaw(t *testing.T) {
	machine := newMemoryCaller()
	account := exampleAccount()
	if _, err := NewAccounts(machine).Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "n", Ciphertext: "c"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewAccountSync(machine, true, t.TempDir())
	scope := accountsync.Scope{Owner: account.Owner, AccountID: account.ID}
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	inboxRaw := []byte("From: Remote <sender@remote.example>\r\nTo: alice@remote.example\r\nSubject: Quarterly report\r\n\r\nprovider body")
	archiveRaw := []byte("From: old@remote.example\r\nTo: alice@remote.example\r\nSubject: Old\r\n\r\narchive")
	_, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{CursorAfter: "ready", Phase: accounts.PhaseHeaders,
		Folders: []accountsync.Folder{{StableID: "INBOX", Revision: "1", Name: "INBOX", Role: "inbox"}, {StableID: "Archive", Revision: "1", Name: "Archive", Role: "archive"}},
		Messages: []accountsync.Message{
			{StableID: "INBOX:1:7", Revision: "1", FolderStableID: "INBOX", Subject: "Quarterly report", Sender: "Remote <sender@remote.example>", To: []string{"alice@remote.example"}, Snippet: "provider body", AttachmentNames: []string{"report.pdf"}, ReceivedAt: now, Flags: []string{"\\Flagged"}, Size: uint64(len(inboxRaw)), RawRFC822: inboxRaw},
			{StableID: "Archive:1:8", Revision: "1", FolderStableID: "Archive", Subject: "Old", Sender: "old@remote.example", ReceivedAt: now.Add(-time.Hour), Flags: []string{"\\Seen"}, Size: uint64(len(archiveRaw)), RawRFC822: archiveRaw},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	unread := false
	starred := true
	listed, err := store.ListUnifiedMessages(context.Background(), account.Owner, accountsync.UnifiedMessageFilter{FolderRole: "inbox", Query: "report.pdf", Read: &unread, Starred: &starred, Limit: 10})
	if err != nil || len(listed) != 1 || listed[0].AccountID != account.ID || listed[0].MailboxAddress != account.MailboxAddress || listed[0].FolderRole != "inbox" || listed[0].RawRFC822 != nil {
		t.Fatalf("ListUnifiedMessages() = %#v, %v", listed, err)
	}
	listed, err = store.ListUnifiedMessages(context.Background(), account.Owner, accountsync.UnifiedMessageFilter{AccountID: account.ID, FolderStableID: "Archive", Limit: 10})
	if err != nil || len(listed) != 1 || listed[0].StableID != "Archive:1:8" {
		t.Fatalf("exact folder list = %#v, %v", listed, err)
	}
	from := now.Add(-2 * time.Hour)
	listed, err = store.ListUnifiedMessages(context.Background(), account.Owner, accountsync.UnifiedMessageFilter{ReceivedAtFrom: &from, OldestFirst: true, Limit: 1})
	if err != nil || len(listed) != 1 || listed[0].StableID != "Archive:1:8" {
		t.Fatalf("oldest replay page = %#v, %v", listed, err)
	}
	from = now.Add(-30 * time.Minute)
	listed, err = store.ListUnifiedMessages(context.Background(), account.Owner, accountsync.UnifiedMessageFilter{ReceivedAtFrom: &from, OldestFirst: true, Limit: 10})
	if err != nil || len(listed) != 1 || listed[0].StableID != "INBOX:1:7" {
		t.Fatalf("replay lower bound = %#v, %v", listed, err)
	}
	message, source, err := store.GetMessageSource(context.Background(), scope, "INBOX:1:7")
	if err != nil || message.Subject != "Quarterly report" || string(source) != string(inboxRaw) {
		t.Fatalf("GetMessageSource() = %#v %q, %v", message, source, err)
	}
	metadata, err := store.GetMessage(context.Background(), scope, "INBOX:1:7")
	if err != nil || metadata.Subject != message.Subject || len(metadata.RawRFC822) != 0 {
		t.Fatalf("GetMessage() = %#v, %v", metadata, err)
	}
	foreign := account.Owner
	foreign.PrincipalID, foreign.Address = "principal-bob", "bob@example.com"
	foreignList, err := store.ListUnifiedMessages(context.Background(), foreign, accountsync.UnifiedMessageFilter{Limit: 10})
	if err != nil || len(foreignList) != 0 {
		t.Fatalf("foreign unified list = %#v, %v", foreignList, err)
	}
	if _, _, err := store.GetMessageSource(context.Background(), accountsync.Scope{Owner: foreign, AccountID: account.ID}, "INBOX:1:7"); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign source error = %v", err)
	}
	if _, err := store.GetMessage(context.Background(), accountsync.Scope{Owner: foreign, AccountID: account.ID}, "INBOX:1:7"); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign message metadata error = %v", err)
	}
}

func TestAccountSyncMutationIdempotencyIsOwnerAndAccountScoped(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	mutation := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{
		ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "action-1", Kind: accountsync.MutationSetRead,
		MessageStableIDs: []string{"INBOX:1"}, Value: true,
	}, Scope: scope, State: accountsync.MutationQueued, CreatedAt: now, UpdatedAt: now}
	first, err := store.EnqueueMutation(context.Background(), scope, mutation)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.EnqueueMutation(context.Background(), scope, mutation)
	if err != nil || first.ID != second.ID || countCollection(machine, ProviderMutations) != 1 {
		t.Fatalf("idempotent enqueue = %#v, %v count=%d", second, err, countCollection(machine, ProviderMutations))
	}
	lease, err := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingMutations(context.Background(), lease, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMutations() = %#v, %v", pending, err)
	}
	pending[0].State = accountsync.MutationApplying
	if err := store.UpdateMutation(context.Background(), lease, pending[0]); err != nil {
		t.Fatalf("UpdateMutation() error = %v", err)
	}
	foreign := scope
	foreign.Owner.PrincipalID = "bob"
	foreign.Owner.Address = "bob@example.com"
	foreignMutation := mutation
	foreignMutation.ID, foreignMutation.Scope = "mut_bbbbbbbbbbbbbbbbbbbb", foreign
	if _, err := store.EnqueueMutation(context.Background(), foreign, foreignMutation); err != nil {
		t.Fatal(err)
	}
	if countCollection(machine, ProviderMutations) != 2 {
		t.Fatal("same-domain owners did not receive distinct journals")
	}
}

func TestAccountSyncProjectsOneGmailMessageIntoEveryLabelWithoutDuplicates(t *testing.T) {
	machine := newMemoryCaller()
	account := exampleAccount()
	account.Provider = accounts.ProviderGmail
	if _, err := NewAccounts(machine).Create(context.Background(), account, accounts.EncryptedSecret{KeyVersion: 1, Nonce: "n", Ciphertext: "c"}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewAccountSync(machine, true, t.TempDir())
	scope := accountsync.Scope{Owner: account.Owner, AccountID: account.ID}
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	raw := []byte("Subject: one Gmail object\r\n\r\nbody")
	folders := []accountsync.Folder{
		{StableID: "INBOX", Revision: "1", Name: "INBOX", Role: "inbox"},
		{StableID: "Projects", Revision: "1", Name: "Projects"},
		{StableID: "[Gmail]/All Mail", Revision: "1", Name: "[Gmail]/All Mail", Role: "all"},
	}
	checkpoint, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{CursorAfter: "gmail-1", Phase: accounts.PhaseHeaders, Folders: folders,
		Messages: []accountsync.Message{{StableID: "gmail:123456", RemoteStableID: "[Gmail]/All Mail:77:9", Revision: "gmail:123456:a", FolderStableID: "[Gmail]/All Mail",
			FolderStableIDs: []string{"INBOX", "Projects", "[Gmail]/All Mail"}, Subject: "one Gmail object", ReceivedAt: now, Flags: []string{}, Size: uint64(len(raw)), RawRFC822: raw}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, filter := range []accountsync.UnifiedMessageFilter{{FolderStableID: "INBOX", Limit: 10}, {FolderStableID: "Projects", Limit: 10}, {FolderRole: "all", Limit: 10}} {
		listed, err := store.ListUnifiedMessages(context.Background(), account.Owner, filter)
		if err != nil || len(listed) != 1 || listed[0].StableID != "gmail:123456" {
			t.Fatalf("filter %#v = %#v, %v", filter, listed, err)
		}
	}
	if countCollection(machine, ProviderMessages) != 1 || countCollection(machine, ProviderRawFiles) != 1 {
		t.Fatalf("Gmail projection duplicated messages=%d raw=%d", countCollection(machine, ProviderMessages), countCollection(machine, ProviderRawFiles))
	}
	checkpoint, err = store.CommitBatch(context.Background(), lease, checkpoint.Version, accountsync.Batch{CursorBefore: checkpoint.Cursor, CursorAfter: "gmail-2", Phase: accounts.PhaseHeaders,
		Updates: []accountsync.MessageUpdate{{StableID: "gmail:123456", RemoteStableID: "[Gmail]/All Mail:77:9", Revision: "gmail:123456:b", FolderStableIDs: []string{"Projects", "[Gmail]/All Mail"}, Flags: []string{"\\Seen"}}}})
	if err != nil {
		t.Fatal(err)
	}
	inbox, _ := store.ListUnifiedMessages(context.Background(), account.Owner, accountsync.UnifiedMessageFilter{FolderStableID: "INBOX", Limit: 10})
	projects, _ := store.ListMessages(context.Background(), scope, "Projects", 10)
	if len(inbox) != 0 || len(projects) != 1 || projects[0].RemoteStableID != "[Gmail]/All Mail:77:9" {
		t.Fatalf("updated memberships inbox=%#v projects=%#v", inbox, projects)
	}
	mutation := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "gmail-inbox", Kind: accountsync.MutationMove,
		MessageStableIDs: []string{"gmail:123456"}, TargetFolderID: "INBOX"}, Scope: scope, State: accountsync.MutationApplying, ResultStableIDs: []string{"gmail:123456"}}
	if err := store.ApplyMutationProjection(context.Background(), lease, mutation); err != nil {
		t.Fatal(err)
	}
	inbox, _ = store.ListUnifiedMessages(context.Background(), account.Owner, accountsync.UnifiedMessageFilter{FolderStableID: "INBOX", Limit: 10})
	if len(inbox) != 1 || inbox[0].FolderStableID != "INBOX" || inbox[0].RemoteStableID != "" || countCollection(machine, ProviderMessages) != 1 {
		t.Fatalf("projected Gmail move = %#v", inbox)
	}
	_, err = store.CommitBatch(context.Background(), lease, checkpoint.Version, accountsync.Batch{CursorBefore: checkpoint.Cursor, CursorAfter: "gmail-3", Phase: accounts.PhaseFolders,
		Folders: []accountsync.Folder{folders[1], folders[2]}, FolderSnapshot: true})
	if err != nil || countCollection(machine, ProviderMessages) != 1 || countCollection(machine, ProviderRawFiles) != 1 {
		t.Fatalf("removed label deleted canonical message: err=%v messages=%d raw=%d", err, countCollection(machine, ProviderMessages), countCollection(machine, ProviderRawFiles))
	}
}

func TestAccountSyncReconcilesRemoteFlagsExpungesAndMutationProjection(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	messages := make([]accountsync.Message, 0, 3)
	for uid := 1; uid <= 3; uid++ {
		raw := []byte("Subject: imported\r\n\r\nbody")
		messages = append(messages, accountsync.Message{StableID: "INBOX:77:" + string(rune('0'+uid)), Revision: "initial", FolderStableID: "INBOX",
			Subject: "imported", ReceivedAt: now, Flags: []string{}, Size: uint64(len(raw)), RawRFC822: raw})
	}
	checkpoint, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{CursorAfter: "initial", Phase: accounts.PhaseHeaders,
		Folders: []accountsync.Folder{{StableID: "INBOX", Revision: "77", Name: "INBOX"}}, Messages: messages})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err = store.CommitBatch(context.Background(), lease, checkpoint.Version, accountsync.Batch{CursorBefore: checkpoint.Cursor, CursorAfter: "delta", Phase: accounts.PhaseHeaders,
		Updates: []accountsync.MessageUpdate{{StableID: "INBOX:77:1", Revision: "remote-flags", Flags: []string{"\\Seen"}}},
		Windows: []accountsync.ReconcileWindow{{FolderStableID: "INBOX", UIDValidity: 77, StartUID: 1, EndUID: 3, PresentIDs: []string{"INBOX:77:1", "INBOX:77:3"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if countCollection(machine, ProviderMessages) != 2 || countCollection(machine, ProviderRawFiles) != 2 {
		t.Fatalf("reconciled counts messages=%d raw=%d", countCollection(machine, ProviderMessages), countCollection(machine, ProviderRawFiles))
	}
	message := documentByStableID(machine, ProviderMessages, "INBOX:77:1")
	if flags := documentStrings(message["flags"]); len(flags) != 1 || flags[0] != "\\Seen" {
		t.Fatalf("remote flags = %#v", message["flags"])
	}
	mutation := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{ID: "mut_abcdefghijklmnopqrst", IdempotencyKey: "star-1",
		Kind: accountsync.MutationSetStarred, MessageStableIDs: []string{"INBOX:77:1"}, Value: true}, Scope: scope, State: accountsync.MutationApplying}
	if err := store.ApplyMutationProjection(context.Background(), lease, mutation); err != nil {
		t.Fatal(err)
	}
	message = documentByStableID(machine, ProviderMessages, "INBOX:77:1")
	if flags := documentStrings(message["flags"]); len(flags) != 2 || flags[0] != "\\Flagged" || flags[1] != "\\Seen" {
		t.Fatalf("projected flags = %#v", message["flags"])
	}
	folderMutation := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationFolderRename,
		FolderStableID: "Projects/Old", FolderName: "New", FolderPath: "Projects/Old", NewFolderPath: "Projects/New"}, Scope: scope}
	if err := store.ApplyMutationProjection(context.Background(), lease, folderMutation); err != nil || countCollection(machine, ProviderMessages) != 2 {
		t.Fatalf("folder projection changed the message index: err=%v count=%d", err, countCollection(machine, ProviderMessages))
	}
	mutation.ID, mutation.Kind = "mut_bbbbbbbbbbbbbbbbbbbb", accountsync.MutationMove
	mutation.TargetFolderID = "Archive"
	mutation.ResultStableIDs = []string{"Archive:88:41"}
	if err := store.ApplyMutationProjection(context.Background(), lease, mutation); err != nil {
		t.Fatal(err)
	}
	moved := documentByStableID(machine, ProviderMessages, "Archive:88:41")
	if documentByStableID(machine, ProviderMessages, "INBOX:77:1") != nil || moved == nil || moved["folderStableId"] != "Archive" || countCollection(machine, ProviderRawFiles) != 2 {
		t.Fatalf("move projection = %#v raw=%d", moved, countCollection(machine, ProviderRawFiles))
	}
}

func TestAccountSyncUIDValidityResetRemovesStaleFolderIndex(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	raw := []byte("Subject: old\r\n\r\nbody")
	checkpoint, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{CursorAfter: "old", Phase: accounts.PhaseHeaders,
		Folders:  []accountsync.Folder{{StableID: "INBOX", Revision: "77", Name: "INBOX"}},
		Messages: []accountsync.Message{{StableID: "INBOX:77:9", Revision: "old", FolderStableID: "INBOX", Subject: "old", ReceivedAt: now, Size: uint64(len(raw)), RawRFC822: raw}}})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err = store.CommitBatch(context.Background(), lease, checkpoint.Version, accountsync.Batch{CursorBefore: checkpoint.Cursor, CursorAfter: "new", Phase: accounts.PhaseFolders,
		Folders: []accountsync.Folder{{StableID: "INBOX", Revision: "88", Name: "INBOX", ResetMessages: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if countCollection(machine, ProviderMessages) != 0 || countCollection(machine, ProviderRawFiles) != 0 {
		t.Fatal("UIDVALIDITY reset retained stale indexed data")
	}
	_, err = store.CommitBatch(context.Background(), lease, checkpoint.Version, accountsync.Batch{CursorBefore: checkpoint.Cursor, CursorAfter: "folder-removed", Phase: accounts.PhaseFolders, FolderSnapshot: true})
	if err != nil {
		t.Fatal(err)
	}
	if countCollection(machine, ProviderFolders) != 0 {
		t.Fatal("complete folder snapshot retained a remotely deleted folder")
	}
}

func TestAccountSyncNewMailNotificationsAreIncrementalDurableAndLeased(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "worker-a", now, time.Minute)
	baseline := []byte("Subject: history\r\n\r\nhistory")
	checkpoint, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{
		CursorAfter: "baseline", Phase: accounts.PhaseBodies,
		Folders:  []accountsync.Folder{{StableID: "INBOX", Revision: "1", Name: "INBOX", Role: "inbox"}},
		Messages: []accountsync.Message{{StableID: "INBOX:1:1", Revision: "1", FolderStableID: "INBOX", Sender: "old@example.net", Subject: "history", ReceivedAt: now.Add(-time.Hour), Flags: []string{}, Size: uint64(len(baseline)), RawRFC822: baseline}},
	})
	if err != nil || countCollection(machine, ProviderNotifications) != 0 {
		t.Fatalf("initial hydration notifications=%d err=%v", countCollection(machine, ProviderNotifications), err)
	}
	fresh := []byte("Subject: fresh\r\n\r\nfresh")
	checkpoint, err = store.CommitBatch(context.Background(), lease, checkpoint.Version, accountsync.Batch{
		CursorBefore: checkpoint.Cursor, CursorAfter: "incremental", Phase: accounts.PhaseBodies,
		Messages: []accountsync.Message{{StableID: "INBOX:1:2", Revision: "2", FolderStableID: "INBOX", Sender: "sender@example.net", Subject: "fresh", ReceivedAt: now, Flags: []string{}, Size: uint64(len(fresh)), RawRFC822: fresh}},
	})
	if err != nil || countCollection(machine, ProviderNotifications) != 1 {
		t.Fatalf("incremental notifications=%d err=%v", countCollection(machine, ProviderNotifications), err)
	}
	claim, err := store.ClaimNewMailNotifications(context.Background(), "push-worker", time.Now().UTC().Add(time.Second), time.Minute, 10)
	if err != nil || claim.Token == "" || len(claim.Notifications) != 1 || claim.Notifications[0].StableID != "INBOX:1:2" || claim.Notifications[0].OwnerAddress != scope.Owner.Address {
		t.Fatalf("ClaimNewMailNotifications() = %#v, %v", claim, err)
	}
	if second, err := store.ClaimNewMailNotifications(context.Background(), "other-worker", time.Now().UTC().Add(time.Second), time.Minute, 10); err != nil || len(second.Notifications) != 0 {
		t.Fatalf("concurrent claim = %#v, %v", second, err)
	}
	if err := store.CompleteNewMailNotifications(context.Background(), claim.Token, []string{claim.Notifications[0].ID}, true, "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if replay, err := store.ClaimNewMailNotifications(context.Background(), "push-worker", time.Now().UTC().Add(2*time.Minute), time.Minute, 10); err != nil || len(replay.Notifications) != 0 {
		t.Fatalf("delivered notification replayed = %#v, %v", replay, err)
	}
	if _, err := store.ClaimNewMailNotifications(context.Background(), "retention-worker", now.Add(31*24*time.Hour), time.Minute, 10); err != nil || countCollection(machine, ProviderNotifications) != 0 {
		t.Fatalf("expired delivered notification retained: count=%d err=%v", countCollection(machine, ProviderNotifications), err)
	}
}

func TestAccountSyncDeleteAccountDataPurgesEveryOwnedSyncRecord(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, false, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, err := store.AcquireLease(context.Background(), scope, "cleanup-worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("Subject: remove me\r\n\r\nbody")
	if _, err := store.CommitBatch(context.Background(), lease, 0, accountsync.Batch{
		CursorAfter: "ready", Phase: accounts.PhaseBodies,
		Folders:  []accountsync.Folder{{StableID: "INBOX", Revision: "1", Name: "INBOX"}},
		Messages: []accountsync.Message{{StableID: "INBOX:1:1", Revision: "1", FolderStableID: "INBOX", Subject: "remove me", ReceivedAt: now, Flags: []string{}, Size: uint64(len(raw)), RawRFC822: raw}},
	}); err != nil {
		t.Fatal(err)
	}
	mutation := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{ID: "mut_cleanupxxxxxxxxxxxxx", IdempotencyKey: "cleanup-1", Kind: accountsync.MutationDelete,
		MessageStableIDs: []string{"INBOX:1:1"}}, Scope: scope, State: accountsync.MutationQueued, CreatedAt: now, UpdatedAt: now}
	if _, err := store.EnqueueMutation(context.Background(), scope, mutation); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAccountData(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{ProviderSyncState, ProviderFolders, ProviderMessages, ProviderNotifications, ProviderRawFiles, ProviderMutations, ProviderContactState, ProviderContactBooks, ProviderContacts, ProviderContactMutations, ProviderCalendarState, ProviderCalendars, ProviderCalendarResources, ProviderCalendarMutations} {
		if got := countCollection(machine, collection); got != 0 {
			t.Fatalf("%s retained %d account-owned records", collection, got)
		}
	}
}

func syncScope() accountsync.Scope {
	return accountsync.Scope{Owner: accounts.Owner{TenantID: "tenant", PrincipalID: "alice", Address: "alice@example.com"}, AccountID: "ext_AAAAAAAAAAAAAAAAAAAA"}
}

func countCollection(machine *memoryCaller, collection string) int {
	count := 0
	for _, document := range machine.documents {
		if document.collection == collection {
			count++
		}
	}
	return count
}

func oneDocument(machine *memoryCaller, collection string) map[string]any {
	for _, document := range machine.documents {
		if document.collection == collection {
			return document.data
		}
	}
	return nil
}

func documentByStableID(machine *memoryCaller, collection, stableID string) map[string]any {
	for _, document := range machine.documents {
		if document.collection == collection && document.data["stableId"] == stableID {
			return document.data
		}
	}
	return nil
}

func documentStrings(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
