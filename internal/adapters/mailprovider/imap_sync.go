package mailprovider

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/platform/imapwire"
)

const (
	defaultSyncFolders      = 250
	defaultSyncMessages     = 500
	defaultSyncMessageBytes = 25 << 20
	defaultSyncPageSize     = 50
	maxDeltaPageSize        = 200
	defaultIdleTimeout      = 25 * time.Minute
	maxIdleTimeout          = 29 * time.Minute
)

var (
	imapUIDPattern         = regexp.MustCompile(`(?i)\bUID\s+(\d+)`)
	imapFlagsPattern       = regexp.MustCompile(`(?i)\bFLAGS\s+\(([^)]*)\)`)
	imapDatePattern        = regexp.MustCompile(`(?i)\bINTERNALDATE\s+"([^"]+)"`)
	imapLiteralPattern     = regexp.MustCompile(`\{(\d+)\}$`)
	imapUIDValidityPattern = regexp.MustCompile(`(?i)\[UIDVALIDITY\s+(\d+)\]`)
	imapUIDNextPattern     = regexp.MustCompile(`(?i)\[UIDNEXT\s+(\d+)\]`)
	imapCopyUIDPattern     = regexp.MustCompile(`(?i)\[COPYUID\s+(\d+)\s+(\d+)\s+(\d+)\]`)
	imapHighestModSeq      = regexp.MustCompile(`(?i)\[HIGHESTMODSEQ\s+(\d+)\]`)
	imapModSeqPattern      = regexp.MustCompile(`(?i)\bMODSEQ\s+\((\d+)\)`)
	imapVanishedPattern    = regexp.MustCompile(`(?i)^\*\s+VANISHED(?:\s+\(EARLIER\))?\s+(.+)$`)
	imapGmailMessageID     = regexp.MustCompile(`(?i)\bX-GM-MSGID\s+(\d+)`)
)

type IMAPSyncOptions struct {
	MaxFolders      int
	MaxMessages     int
	MaxMessageBytes int
	PageSize        int
	IdleTimeout     time.Duration
}

// IMAPConnector implements bounded UID-based initial and incremental message
// import. The cursor is committed by the caller only after every returned raw
// object is durable in FYLO.
type IMAPConnector struct {
	network         *PasswordValidator
	maxFolders      int
	maxMessages     int
	maxMessageBytes int
	pageSize        int
	idleTimeout     time.Duration
}

func NewIMAPConnector(network *PasswordValidator, options IMAPSyncOptions) (*IMAPConnector, error) {
	if network == nil {
		return nil, errors.New("IMAP connector network policy is required")
	}
	if options.MaxFolders <= 0 {
		options.MaxFolders = defaultSyncFolders
	}
	if options.MaxMessages <= 0 {
		options.MaxMessages = defaultSyncMessages
	}
	if options.MaxMessageBytes <= 0 {
		options.MaxMessageBytes = defaultSyncMessageBytes
	}
	if options.PageSize <= 0 {
		options.PageSize = defaultSyncPageSize
	}
	if options.IdleTimeout <= 0 {
		options.IdleTimeout = defaultIdleTimeout
	}
	if options.MaxFolders > accountsync.MaxBatchFolders || options.MaxMessages > 10_000 || options.MaxMessageBytes > accountsync.MaxRawBatchBytes || options.PageSize > maxDeltaPageSize || options.IdleTimeout > maxIdleTimeout {
		return nil, errors.New("IMAP connector limits exceed synchronization bounds")
	}
	return &IMAPConnector{network: network, maxFolders: options.MaxFolders, maxMessages: options.MaxMessages, maxMessageBytes: options.MaxMessageBytes, pageSize: options.PageSize, idleTimeout: options.IdleTimeout}, nil
}

func (connector *IMAPConnector) WaitForChange(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials) (bool, error) {
	session, err := connector.open(ctx, account, credentials)
	if err != nil {
		return false, err
	}
	defer session.close()
	if !session.idle {
		return false, accountsync.ErrWatchUnsupported
	}
	folders, err := session.listFolders()
	if err != nil {
		return false, err
	}
	inbox := inboxFolder(folders)
	if _, err := session.selectFolder(inbox); err != nil {
		return false, err
	}
	return session.waitForChange(ctx, connector.idleTimeout)
}

func inboxFolder(folders []accountsync.Folder) string {
	for _, folder := range folders {
		if folder.Role == "inbox" {
			return folder.StableID
		}
	}
	return "INBOX"
}

func (connector *IMAPConnector) Sync(ctx context.Context, request accountsync.SyncRequest, sink accountsync.ChangeSink) error {
	session, err := connector.open(ctx, request.Account, request.Credentials)
	if err != nil {
		return err
	}
	defer session.close()
	cursor, err := decodeIMAPCursor(request.Checkpoint.Cursor)
	if err != nil {
		return safeConnectorError(accountsync.FailurePermanent, "invalid_sync_cursor", "The saved IMAP sync cursor is invalid.")
	}
	folders, err := connector.prepareFolders(session, cursor)
	if err != nil {
		return err
	}
	folderCursor, err := encodeIMAPCursor(cursor)
	if err != nil {
		return err
	}
	if err := commitFolderSnapshot(ctx, sink, request.Checkpoint.Cursor, folderCursor, folders); err != nil {
		return err
	}
	session.gmailFolders = gmailFolderMap(request.Account.Provider, folders)
	syncFolders := providerSyncFolders(request.Account.Provider, folders)
	run := imapSyncRun{cursor: cursor, currentCursor: folderCursor}
	for _, folder := range syncFolders {
		if err := connector.syncFolder(ctx, session, folder, &run, sink); err != nil {
			return err
		}
		if run.completed >= connector.maxMessages {
			break
		}
	}
	return connector.reconcileChanges(ctx, session, syncFolders, run.cursor, run.currentCursor, run.completed, sink)
}

func providerSyncFolders(provider accounts.Provider, folders []accountsync.Folder) []accountsync.Folder {
	if provider != accounts.ProviderGmail {
		return folders
	}
	result := make([]accountsync.Folder, 0, 3)
	for _, folder := range folders {
		if folder.Role == "all" || folder.Role == "spam" || folder.Role == "trash" {
			result = append(result, folder)
		}
	}
	if len(result) > 0 {
		priority := map[string]int{"all": 0, "spam": 1, "trash": 2}
		sort.SliceStable(result, func(left, right int) bool { return priority[result[left].Role] < priority[result[right].Role] })
		return result
	}
	for _, folder := range folders {
		if folder.Role == "inbox" {
			return []accountsync.Folder{folder}
		}
	}
	return folders
}

func gmailFolderMap(provider accounts.Provider, folders []accountsync.Folder) map[string]string {
	if provider != accounts.ProviderGmail {
		return nil
	}
	result := make(map[string]string, len(folders)*3)
	for _, folder := range folders {
		result[strings.ToLower(folder.StableID)] = folder.StableID
		result[strings.ToLower(folder.Name)] = folder.StableID
		if folder.Role != "" {
			result["\\"+strings.ToLower(folder.Role)] = folder.StableID
		}
		switch folder.Role {
		case "all", "archive":
			result["\\allmail"] = folder.StableID
		case "spam":
			result["\\junk"] = folder.StableID
		case "sent":
			result["\\sentmail"] = folder.StableID
		case "drafts":
			result["\\draft"] = folder.StableID
		}
	}
	return result
}

type imapSyncRun struct {
	cursor        imapCursor
	currentCursor string
	completed     int
}

func (connector *IMAPConnector) prepareFolders(session *imapSession, cursor imapCursor) ([]accountsync.Folder, error) {
	folders, err := session.listFolders()
	if err != nil {
		return nil, err
	}
	if len(folders) > connector.maxFolders {
		return nil, safeConnectorError(accountsync.FailurePermanent, "folder_limit_exceeded", "The IMAP account has more folders than this server permits.")
	}
	syncIDs := map[string]bool{}
	for _, folder := range providerSyncFolders(session.provider, folders) {
		syncIDs[folder.StableID] = true
	}
	for index := range folders {
		if session.provider == accounts.ProviderGmail && !syncIDs[folders[index].StableID] {
			folders[index].Revision = "gmail-label"
			delete(cursor.Folders, folders[index].StableID)
			continue
		}
		selected, err := session.selectFolder(folders[index].StableID)
		if err != nil {
			return nil, err
		}
		folders[index].Revision = strconv.FormatUint(selected.uidValidity, 10)
		state := cursor.Folders[folders[index].StableID]
		if state.UIDValidity != selected.uidValidity {
			folders[index].ResetMessages = state.UIDValidity != 0
			state = imapFolderCursor{UIDValidity: selected.uidValidity, ReconcileUID: 1}
		}
		cursor.Folders[folders[index].StableID] = state
	}
	return folders, nil
}

func commitFolderSnapshot(ctx context.Context, sink accountsync.ChangeSink, before, after string, folders []accountsync.Folder) error {
	if before == after && len(folders) == 0 {
		return nil
	}
	return sink.Commit(ctx, accountsync.Batch{CursorBefore: before, CursorAfter: after, Phase: accounts.PhaseFolders, Folders: folders, FolderSnapshot: true})
}

func (connector *IMAPConnector) syncFolder(ctx context.Context, session *imapSession, folder accountsync.Folder, run *imapSyncRun, sink accountsync.ChangeSink) error {
	state := run.cursor.Folders[folder.StableID]
	selected, err := session.selectFolderChanges(folder.StableID, state)
	if err != nil {
		return err
	}
	if state.UIDValidity != selected.uidValidity {
		state = imapFolderCursor{UIDValidity: selected.uidValidity, ReconcileUID: 1}
		selected.qresync = false
	}
	if err := commitQResyncChanges(ctx, folder, selected, &state, run, sink); err != nil {
		return err
	}
	uids, err := session.searchUIDs(state.LastUID + 1)
	if err != nil {
		return err
	}
	uids = boundedUIDs(uids, connector.maxMessages-run.completed)
	if err := connector.importUIDs(ctx, session, folder, selected, &state, uids, run, sink); err != nil {
		return err
	}
	return advanceEmptyMailbox(ctx, folder, selected, state, uids, run, sink)
}

func commitQResyncChanges(ctx context.Context, folder accountsync.Folder, selected selectedFolder, state *imapFolderCursor, run *imapSyncRun, sink accountsync.ChangeSink) error {
	if !selected.qresync {
		return nil
	}
	updates := append([]accountsync.MessageUpdate(nil), selected.updates...)
	deletions := qresyncDeletions(folder.StableID, *state, selected)
	state.HighestModSeq, state.ModSeqTarget, state.ModSeqUID = selected.highestModSeq, 0, 0
	run.cursor.Folders[folder.StableID] = *state
	nextCursor, err := encodeIMAPCursor(run.cursor)
	if err != nil {
		return err
	}
	if nextCursor == run.currentCursor && len(updates) == 0 && len(deletions) == 0 {
		return nil
	}
	if err := sink.Commit(ctx, accountsync.Batch{CursorBefore: run.currentCursor, CursorAfter: nextCursor, Phase: accounts.PhaseHeaders, Completed: uint64(run.completed), Updates: updates, Deletions: deletions}); err != nil {
		return err
	}
	run.currentCursor = nextCursor
	return nil
}

func qresyncDeletions(folder string, state imapFolderCursor, selected selectedFolder) []accountsync.Deletion {
	deletions := make([]accountsync.Deletion, 0, len(selected.vanishedUIDs))
	for _, uid := range selected.vanishedUIDs {
		if uid <= state.LastUID {
			deletions = append(deletions, accountsync.Deletion{StableID: fmt.Sprintf("%s:%d:%d", folder, state.UIDValidity, uid), Revision: strconv.FormatUint(selected.highestModSeq, 10)})
		}
	}
	return deletions
}

func boundedUIDs(uids []uint64, available int) []uint64 {
	if len(uids) > available {
		return uids[:available]
	}
	return uids
}

func (connector *IMAPConnector) importUIDs(ctx context.Context, session *imapSession, folder accountsync.Folder, selected selectedFolder, state *imapFolderCursor, uids []uint64, run *imapSyncRun, sink accountsync.ChangeSink) error {
	for start := 0; start < len(uids); start += connector.pageSize {
		end := min(start+connector.pageSize, len(uids))
		messages, err := connector.fetchMessagePage(session, folder.StableID, selected.uidValidity, uids[start:end], state)
		if err != nil {
			return err
		}
		run.cursor.Folders[folder.StableID] = *state
		nextCursor, err := encodeIMAPCursor(run.cursor)
		if err != nil {
			return err
		}
		run.completed += len(messages)
		if err := sink.Commit(ctx, accountsync.Batch{CursorBefore: run.currentCursor, CursorAfter: nextCursor, Phase: accounts.PhaseHeaders, Completed: uint64(run.completed), Messages: messages}); err != nil {
			return err
		}
		run.currentCursor = nextCursor
	}
	return nil
}

func (connector *IMAPConnector) fetchMessagePage(session *imapSession, folder string, uidValidity uint64, uids []uint64, state *imapFolderCursor) ([]accountsync.Message, error) {
	messages := make([]accountsync.Message, 0, len(uids))
	for _, uid := range uids {
		message, err := session.fetchMessage(folder, uidValidity, uid, connector.maxMessageBytes)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
		state.LastUID = uid
	}
	return messages, nil
}

// advanceEmptyMailbox uses UIDNEXT so an empty or unchanged mailbox does not
// rescan the same range after a restart; it does not assume contiguous UIDs.
func advanceEmptyMailbox(ctx context.Context, folder accountsync.Folder, selected selectedFolder, state imapFolderCursor, uids []uint64, run *imapSyncRun, sink accountsync.ChangeSink) error {
	if len(uids) != 0 || selected.uidNext == 0 || selected.uidNext-1 <= state.LastUID {
		return nil
	}
	state.LastUID = selected.uidNext - 1
	run.cursor.Folders[folder.StableID] = state
	nextCursor, err := encodeIMAPCursor(run.cursor)
	if err != nil {
		return err
	}
	if err := sink.Commit(ctx, accountsync.Batch{CursorBefore: run.currentCursor, CursorAfter: nextCursor, Phase: accounts.PhaseHeaders, Completed: uint64(run.completed)}); err != nil {
		return err
	}
	run.currentCursor = nextCursor
	return nil
}

func (connector *IMAPConnector) reconcileChanges(ctx context.Context, session *imapSession, folders []accountsync.Folder, cursor imapCursor, currentCursor string, imported int, sink accountsync.ChangeSink) error {
	if len(folders) == 0 {
		return nil
	}
	budget := connector.maxMessages
	for visited := 0; visited < len(folders) && budget > 0; visited++ {
		index := cursor.ReconcileFolder % len(folders)
		folder := folders[index]
		state := cursor.Folders[folder.StableID]
		cursor.ReconcileFolder = (index + 1) % len(folders)
		window, ok, err := connector.planReconcileWindow(session, folder, state, budget)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		updates, present, err := session.fetchReconcileWindow(window)
		if err != nil {
			return err
		}
		cursor.Folders[folder.StableID] = advanceReconcileCursor(window)
		nextCursor, err := encodeIMAPCursor(cursor)
		if err != nil {
			return err
		}
		scanned := int(window.end-window.start) + 1
		budget -= scanned
		batch := accountsync.Batch{CursorBefore: currentCursor, CursorAfter: nextCursor, Phase: accounts.PhaseHeaders, Completed: uint64(imported + connector.maxMessages - budget),
			Updates: updates, Windows: []accountsync.ReconcileWindow{{FolderStableID: folder.StableID, UIDValidity: window.selected.uidValidity, StartUID: window.start, EndUID: window.end, PresentIDs: present}}}
		if err := sink.Commit(ctx, batch); err != nil {
			return err
		}
		currentCursor = nextCursor
	}
	return nil
}

type imapReconcileWindow struct {
	folder     accountsync.Folder
	selected   selectedFolder
	state      imapFolderCursor
	maximumUID uint64
	start      uint64
	end        uint64
	baseline   uint64
	modSeqScan bool
}

func (connector *IMAPConnector) planReconcileWindow(session *imapSession, folder accountsync.Folder, state imapFolderCursor, budget int) (imapReconcileWindow, bool, error) {
	selected, err := session.selectFolder(folder.StableID)
	if err != nil {
		return imapReconcileWindow{}, false, err
	}
	if modSeqRegressed(selected, state) {
		return imapReconcileWindow{}, false, safeConnectorError(accountsync.FailurePermanent, "imap_modseq_regressed", "The IMAP server returned a regressed mailbox modification sequence.")
	}
	maximumUID := maximumSelectedUID(selected, state.LastUID)
	if maximumUID == 0 || session.qresync && state.HighestModSeq > 0 {
		return imapReconcileWindow{}, false, nil
	}
	width := min(connector.pageSize, budget)
	window := imapReconcileWindow{folder: folder, selected: selected, state: state, maximumUID: maximumUID, baseline: state.HighestModSeq}
	window.modSeqScan = session.condstore && selected.highestModSeq > 0
	window.start = prepareModSeqScan(&window.state, selected, maximumUID, window.modSeqScan)
	window.end = min(window.start+uint64(width)-1, maximumUID)
	return window, true, nil
}

func prepareModSeqScan(state *imapFolderCursor, selected selectedFolder, maximumUID uint64, enabled bool) uint64 {
	start := state.ReconcileUID
	if enabled {
		if state.ModSeqTarget == 0 && selected.highestModSeq > state.HighestModSeq {
			state.ModSeqTarget, state.ModSeqUID = selected.highestModSeq, 1
		}
		if state.ModSeqTarget > 0 {
			start = state.ModSeqUID
		}
	}
	if start == 0 || start > maximumUID {
		return 1
	}
	return start
}

func (session *imapSession) fetchReconcileWindow(window imapReconcileWindow) ([]accountsync.MessageUpdate, []string, error) {
	if window.modSeqScan {
		updates, err := session.fetchMessageUpdatesSince(window.folder.StableID, window.selected.uidValidity, window.start, window.end, window.baseline)
		if err != nil {
			return nil, nil, err
		}
		present, err := session.searchUIDRange(window.folder.StableID, window.selected.uidValidity, window.start, window.end)
		return updates, present, err
	}
	updates, err := session.fetchMessageUpdates(window.folder.StableID, window.selected.uidValidity, window.start, window.end)
	present := make([]string, 0, len(updates))
	for _, update := range updates {
		present = append(present, update.StableID)
	}
	return updates, present, err
}

func advanceReconcileCursor(window imapReconcileWindow) imapFolderCursor {
	state := window.state
	state.ReconcileUID = window.end + 1
	if state.ReconcileUID > window.maximumUID {
		state.ReconcileUID = 1
	}
	if !window.modSeqScan || state.ModSeqTarget == 0 {
		return state
	}
	state.ModSeqUID = window.end + 1
	if state.ModSeqUID > window.maximumUID {
		state.HighestModSeq, state.ModSeqTarget, state.ModSeqUID = state.ModSeqTarget, 0, 0
	}
	return state
}

func maximumSelectedUID(selected selectedFolder, lastUID uint64) uint64 {
	if selected.uidNext > 0 && selected.uidNext-1 > lastUID {
		return selected.uidNext - 1
	}
	return lastUID
}

func modSeqRegressed(selected selectedFolder, state imapFolderCursor) bool {
	return selected.uidValidity == state.UIDValidity && selected.highestModSeq > 0 && selected.highestModSeq < state.HighestModSeq
}

func (connector *IMAPConnector) ApplyMutation(ctx context.Context, request accountsync.MutationContext) (accountsync.MutationResult, error) {
	return connector.mutate(ctx, request, false)
}

func (connector *IMAPConnector) ReconcileMutation(ctx context.Context, request accountsync.MutationContext) (accountsync.MutationResult, error) {
	return connector.mutate(ctx, request, true)
}

func (connector *IMAPConnector) mutate(ctx context.Context, request accountsync.MutationContext, reconcile bool) (accountsync.MutationResult, error) {
	session, err := connector.open(ctx, request.Account, request.Credentials)
	if err != nil {
		return accountsync.MutationResult{}, err
	}
	defer session.close()
	return session.mutateRequest(request, reconcile)
}

func (session *imapSession) mutateRequest(request accountsync.MutationContext, reconcile bool) (accountsync.MutationResult, error) {
	mutation := request.Mutation
	if accountsync.IsFolderMutation(mutation.Kind) {
		return session.mutateFolder(mutation, reconcile)
	}
	if request.Account.Provider == accounts.ProviderGmail {
		return session.mutateGmail(mutation, reconcile)
	}
	if reconcile && mutation.Kind == accountsync.MutationMove {
		return session.reconcileMove(mutation)
	}
	return session.mutateStandardMessages(mutation, reconcile)
}

type mutationTarget struct {
	stableID    string
	uidValidity uint64
	uid         uint64
}

func (session *imapSession) mutateStandardMessages(mutation accountsync.Mutation, reconcile bool) (accountsync.MutationResult, error) {
	grouped := map[string][]mutationTarget{}
	for _, stableID := range mutation.MessageStableIDs {
		folder, uidValidity, uid, ok := parseMessageStableID(stableID)
		if !ok {
			return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "invalid_message_identity"}, nil
		}
		grouped[folder] = append(grouped[folder], mutationTarget{stableID: stableID, uidValidity: uidValidity, uid: uid})
	}
	replacements := make(map[string]string, len(mutation.MessageStableIDs))
	for folder, targets := range grouped {
		result, err := session.mutateStandardFolder(mutation, folder, targets, reconcile, replacements)
		if err != nil || result != nil {
			return resultOrEmpty(result), err
		}
	}
	result := accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}
	if mutation.Kind == accountsync.MutationMove {
		result.ResultStableIDs = make([]string, 0, len(mutation.MessageStableIDs))
		for _, stableID := range mutation.MessageStableIDs {
			replacement, ok := replacements[stableID]
			if !ok {
				return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "imap_copyuid_missing"}, nil
			}
			result.ResultStableIDs = append(result.ResultStableIDs, replacement)
		}
	}
	return result, nil
}

func (session *imapSession) mutateStandardFolder(mutation accountsync.Mutation, folder string, targets []mutationTarget, reconcile bool, replacements map[string]string) (*accountsync.MutationResult, error) {
	selected, err := session.selectFolder(folder)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		result, err := session.mutateStandardTarget(mutation, target, selected.uidValidity, reconcile)
		if err != nil {
			return nil, err
		}
		if result.Outcome != accountsync.OutcomeApplied {
			return &result, nil
		}
		if len(result.ResultStableIDs) == 1 {
			replacements[target.stableID] = result.ResultStableIDs[0]
		}
	}
	return nil, nil
}

func (session *imapSession) mutateStandardTarget(mutation accountsync.Mutation, target mutationTarget, selectedValidity uint64, reconcile bool) (accountsync.MutationResult, error) {
	if target.uidValidity != selectedValidity {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "imap_uidvalidity_changed"}, nil
	}
	if !reconcile {
		return session.apply(mutation, target.uid)
	}
	applied, err := session.reconcile(mutation, target.uid)
	if err != nil {
		return accountsync.MutationResult{}, err
	}
	if !applied {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "mutation_not_applied"}, nil
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
}

func resultOrEmpty(result *accountsync.MutationResult) accountsync.MutationResult {
	if result == nil {
		return accountsync.MutationResult{}
	}
	return *result
}

type gmailMutationTarget struct {
	folder accountsync.Folder
	uid    uint64
}

func (session *imapSession) mutateGmail(mutation accountsync.Mutation, reconcile bool) (accountsync.MutationResult, error) {
	folders, err := session.listFolders()
	if err != nil {
		return accountsync.MutationResult{}, err
	}
	for _, stableID := range mutation.MessageStableIDs {
		result, err := session.mutateGmailMessage(mutation, stableID, folders, reconcile)
		if err != nil || result.Outcome != accountsync.OutcomeApplied {
			return result, err
		}
	}
	result := accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}
	if mutation.Kind == accountsync.MutationMove {
		result.ResultStableIDs = append([]string(nil), mutation.MessageStableIDs...)
	}
	return result, nil
}

func (session *imapSession) mutateGmailMessage(mutation accountsync.Mutation, stableID string, folders []accountsync.Folder, reconcile bool) (accountsync.MutationResult, error) {
	messageID, ok := parseGmailStableID(stableID)
	if !ok {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "invalid_message_identity"}, nil
	}
	target, found, err := session.resolveGmailMessage(messageID, folders)
	if err != nil {
		return accountsync.MutationResult{}, err
	}
	if !reconcile && !found {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "gmail_message_missing"}, nil
	}
	if !reconcile {
		return session.applyGmailMutation(mutation, target, folders)
	}
	return session.reconcileGmailMessage(mutation, target, found, folders)
}

func (session *imapSession) reconcileGmailMessage(mutation accountsync.Mutation, target gmailMutationTarget, found bool, folders []accountsync.Folder) (accountsync.MutationResult, error) {
	applied, err := session.reconcileGmailMutation(mutation, target, found, folders)
	if err != nil {
		return accountsync.MutationResult{}, err
	}
	if !applied {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "mutation_not_applied"}, nil
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
}

func (session *imapSession) resolveGmailMessage(messageID uint64, folders []accountsync.Folder) (gmailMutationTarget, bool, error) {
	for _, folder := range providerSyncFolders(accounts.ProviderGmail, folders) {
		if _, err := session.selectFolder(folder.StableID); err != nil {
			return gmailMutationTarget{}, false, err
		}
		status, lines, err := session.command("UID SEARCH X-GM-MSGID " + strconv.FormatUint(messageID, 10))
		if err != nil {
			return gmailMutationTarget{}, false, err
		}
		if status != "OK" {
			return gmailMutationTarget{}, false, safeConnectorError(accountsync.FailurePermanent, "gmail_search_failed", "Gmail refused a stable message identity search.")
		}
		uids := parseSearchUIDs(lines)
		if len(uids) > 1 {
			return gmailMutationTarget{}, false, safeConnectorError(accountsync.FailurePermanent, "gmail_identity_ambiguous", "Gmail returned more than one message for a stable identity.")
		}
		if len(uids) == 1 {
			return gmailMutationTarget{folder: folder, uid: uids[0]}, true, nil
		}
	}
	return gmailMutationTarget{}, false, nil
}

func parseSearchUIDs(lines []string) []uint64 {
	result := []uint64{}
	for _, line := range lines {
		if !strings.HasPrefix(strings.ToUpper(line), "* SEARCH") {
			continue
		}
		for _, field := range strings.Fields(line)[2:] {
			if uid, err := strconv.ParseUint(field, 10, 64); err == nil && uid > 0 {
				result = append(result, uid)
			}
		}
	}
	return result
}

func (session *imapSession) applyGmailMutation(mutation accountsync.Mutation, target gmailMutationTarget, folders []accountsync.Folder) (accountsync.MutationResult, error) {
	if mutation.Kind != accountsync.MutationMove {
		return session.apply(mutation, target.uid)
	}
	destination, ok := folderByStableID(folders, mutation.TargetFolderID)
	if !ok {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "gmail_label_missing"}, nil
	}
	commands := gmailMoveCommands(target.uid, destination)
	for _, command := range commands {
		status, _, err := session.command(command)
		if err != nil || status != "OK" {
			return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "gmail_label_outcome_unknown"}, nil
		}
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
}

func gmailMoveCommands(uid uint64, destination accountsync.Folder) []string {
	commands := []string{}
	label := gmailFolderLabel(destination)
	if label != "" {
		commands = append(commands, fmt.Sprintf("UID STORE %d +X-GM-LABELS.SILENT (%s)", uid, imapQuote(label)))
	}
	removed := gmailRemovedLabels(destination)
	if len(removed) > 0 {
		quoted := make([]string, 0, len(removed))
		for _, candidate := range removed {
			quoted = append(quoted, imapQuote(candidate))
		}
		commands = append(commands, fmt.Sprintf("UID STORE %d -X-GM-LABELS.SILENT (%s)", uid, strings.Join(quoted, " ")))
	}
	return commands
}

func gmailRemovedLabels(destination accountsync.Folder) []string {
	switch destination.Role {
	case "inbox":
		return []string{"\\Trash", "\\Spam"}
	case "trash":
		return []string{"\\Inbox", "\\Spam"}
	case "spam":
		return []string{"\\Inbox", "\\Trash"}
	default:
		return []string{"\\Inbox", "\\Trash", "\\Spam"}
	}
}

func gmailFolderLabel(folder accountsync.Folder) string {
	if label, known := map[string]string{
		"all": "", "archive": "", "inbox": "\\Inbox", "trash": "\\Trash", "spam": "\\Spam",
		"sent": "\\Sent", "drafts": "\\Draft", "important": "\\Important",
	}[folder.Role]; known {
		return label
	}
	return folder.StableID
}

func folderByStableID(folders []accountsync.Folder, stableID string) (accountsync.Folder, bool) {
	for _, folder := range folders {
		if folder.StableID == stableID {
			return folder, true
		}
	}
	return accountsync.Folder{}, false
}

func (session *imapSession) reconcileGmailMutation(mutation accountsync.Mutation, target gmailMutationTarget, found bool, folders []accountsync.Folder) (bool, error) {
	if mutation.Kind == accountsync.MutationDelete {
		return !found, nil
	}
	if !found {
		return false, nil
	}
	if mutation.Kind != accountsync.MutationMove {
		return session.reconcile(mutation, target.uid)
	}
	destination, ok := folderByStableID(folders, mutation.TargetFolderID)
	if !ok {
		return false, nil
	}
	status, lines, err := session.command(fmt.Sprintf("UID FETCH %d (X-GM-LABELS)", target.uid))
	if err != nil || status != "OK" {
		return false, err
	}
	labels := []string{}
	for _, line := range lines {
		labels = append(labels, parseGmailLabels(line)...)
	}
	return gmailMoveApplied(labels, destination), nil
}

func gmailMoveApplied(labels []string, destination accountsync.Folder) bool {
	wanted := gmailFolderLabel(destination)
	if wanted != "" && !containsFold(labels, wanted) {
		return false
	}
	for _, removed := range gmailRemovedLabels(destination) {
		if containsFold(labels, removed) {
			return false
		}
	}
	return true
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func parseGmailStableID(stableID string) (uint64, bool) {
	if !strings.HasPrefix(stableID, "gmail:") {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(stableID, "gmail:"), 10, 64)
	return value, err == nil && value > 0
}

func (session *imapSession) mutateFolder(mutation accountsync.Mutation, reconcile bool) (accountsync.MutationResult, error) {
	folders, err := session.listFolders()
	if err != nil {
		return accountsync.MutationResult{}, err
	}
	oldPath, newPath := mutation.FolderPath, mutation.NewFolderPath
	if reconcile {
		exists := make(map[string]bool, len(folders))
		for _, folder := range folders {
			exists[folder.StableID] = true
		}
		applied := mutation.Kind == accountsync.MutationFolderCreate && exists[newPath] ||
			mutation.Kind == accountsync.MutationFolderRename && !exists[oldPath] && exists[newPath] ||
			mutation.Kind == accountsync.MutationFolderDelete && !exists[oldPath]
		if applied {
			return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
		}
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "mutation_not_applied"}, nil
	}

	command := ""
	switch mutation.Kind {
	case accountsync.MutationFolderCreate:
		command = "CREATE " + imapQuote(newPath)
	case accountsync.MutationFolderRename:
		command = "RENAME " + imapQuote(oldPath) + " " + imapQuote(newPath)
	case accountsync.MutationFolderDelete:
		command = "DELETE " + imapQuote(oldPath)
	}
	status, _, err := session.command(command)
	if err != nil {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown}, nil
	}
	if status == "OK" {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "imap_folder_mutation_rejected"}, nil
}

type imapCursor struct {
	Folders         map[string]imapFolderCursor `json:"folders"`
	ReconcileFolder int                         `json:"reconcileFolder,omitempty"`
}
type imapFolderCursor struct {
	UIDValidity   uint64 `json:"uidValidity"`
	LastUID       uint64 `json:"lastUid"`
	ReconcileUID  uint64 `json:"reconcileUid,omitempty"`
	HighestModSeq uint64 `json:"highestModSeq,omitempty"`
	ModSeqTarget  uint64 `json:"modSeqTarget,omitempty"`
	ModSeqUID     uint64 `json:"modSeqUid,omitempty"`
}

func decodeIMAPCursor(value string) (imapCursor, error) {
	result := imapCursor{Folders: map[string]imapFolderCursor{}}
	if value == "" {
		return result, nil
	}
	if len(value) > 4096 || json.Unmarshal([]byte(value), &result) != nil || !validIMAPCursor(result) {
		return imapCursor{}, errors.New("invalid IMAP cursor")
	}
	return result, nil
}

func validIMAPCursor(cursor imapCursor) bool {
	if cursor.Folders == nil || len(cursor.Folders) > defaultSyncFolders || cursor.ReconcileFolder < 0 || cursor.ReconcileFolder > defaultSyncFolders {
		return false
	}
	for folder, state := range cursor.Folders {
		if !validIMAPFolderCursor(folder, state) {
			return false
		}
	}
	return true
}

func validIMAPFolderCursor(folder string, state imapFolderCursor) bool {
	if strings.TrimSpace(folder) == "" || len(folder) > 1024 || strings.ContainsAny(folder, "\x00\r\n") {
		return false
	}
	if state.UIDValidity == 0 {
		return emptyIMAPFolderCursor(state)
	}
	return validModSeqCursor(state)
}

func emptyIMAPFolderCursor(state imapFolderCursor) bool {
	return state.LastUID == 0 && state.ReconcileUID == 0 && state.HighestModSeq == 0 && state.ModSeqTarget == 0 && state.ModSeqUID == 0
}

func validModSeqCursor(state imapFolderCursor) bool {
	paired := (state.ModSeqTarget == 0) == (state.ModSeqUID == 0)
	ordered := state.ModSeqTarget == 0 || state.ModSeqTarget > state.HighestModSeq
	return paired && ordered
}

func encodeIMAPCursor(value imapCursor) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > 4096 {
		return "", errors.New("IMAP cursor exceeds durable bound")
	}
	return string(encoded), nil
}

type imapSession struct {
	connection   net.Conn
	reader       *bufio.Reader
	tag          uint64
	capabilities map[string]bool
	condstore    bool
	qresync      bool
	idle         bool
	provider     accounts.Provider
	gmailFolders map[string]string
}

func (connector *IMAPConnector) open(ctx context.Context, account accounts.ExternalAccount, credentials accounts.Credentials) (*imapSession, error) {
	if !validIMAPAccess(account, credentials) {
		return nil, accounts.ErrInvalid
	}
	session, err := connector.connectSession(ctx, account)
	if err != nil {
		return nil, err
	}
	if err := session.authenticateAndNegotiate(account, credentials); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}

func (connector *IMAPConnector) connectSession(ctx context.Context, account accounts.ExternalAccount) (*imapSession, error) {
	endpoint := account.Connection.IMAP
	connection, err := connector.network.connect(ctx, endpoint)
	if err != nil {
		return nil, safeConnectorError(accountsync.FailureTransient, "imap_connection_failed", "CADUCEUS could not connect to the IMAP server.")
	}
	session := &imapSession{connection: connection, reader: protocolReader(connection), provider: account.Provider}
	if !imapGreeting(session.reader) {
		connection.Close()
		return nil, safeConnectorError(accountsync.FailurePermanent, "imap_protocol_failed", "The IMAP server returned an invalid greeting.")
	}
	if endpoint.Security == accounts.SecurityStartTLS {
		connection, reader, err := connector.network.startIMAPTLS(ctx, connection, session.reader, endpoint)
		if err != nil {
			session.close()
			return nil, safeConnectorError(accountsync.FailureTransient, "imap_tls_failed", "The IMAP TLS negotiation failed.")
		}
		session.connection, session.reader = connection, reader
	}
	return session, nil
}

func (session *imapSession) authenticateAndNegotiate(account accounts.ExternalAccount, credentials accounts.Credentials) error {
	status, err := session.authenticate(account, credentials)
	if err != nil {
		return err
	}
	if status != "OK" {
		return safeConnectorError(accountsync.FailureAuthentication, "imap_authentication_failed", "The IMAP server rejected the saved credential.")
	}
	return session.negotiateCapabilities()
}

func validIMAPAccess(account accounts.ExternalAccount, credentials accounts.Credentials) bool {
	if account.Connection.IMAP == nil {
		return false
	}
	if oauthMailProvider(account.Provider) {
		return credentials.AccessToken != ""
	}
	return account.Provider == accounts.ProviderIMAP && credentials.Password != ""
}

func (session *imapSession) authenticate(account accounts.ExternalAccount, credentials accounts.Credentials) (string, error) {
	if oauthMailProvider(account.Provider) {
		status, _, err := session.command("AUTHENTICATE XOAUTH2 " + xoauth2(account.MailboxAddress, credentials.AccessToken))
		return status, err
	}
	username := strings.TrimSpace(account.Connection.IMAP.Username)
	if username == "" {
		username = strings.TrimSpace(credentials.Username)
	}
	if username == "" {
		username = account.MailboxAddress
	}
	status, _, err := session.command("LOGIN " + imapQuote(username) + " " + imapQuote(credentials.Password))
	return status, err
}

func (session *imapSession) negotiateCapabilities() error {
	status, lines, err := session.command("CAPABILITY")
	if err != nil {
		return err
	}
	if status != "OK" {
		if session.provider == accounts.ProviderGmail {
			return safeConnectorError(accountsync.FailurePermanent, "gmail_extension_missing", "Gmail did not advertise its required stable identity and label extension.")
		}
		return nil
	}
	session.capabilities = capabilitySet(lines)
	if session.provider == accounts.ProviderGmail && !session.capabilities["X-GM-EXT-1"] {
		return safeConnectorError(accountsync.FailurePermanent, "gmail_extension_missing", "Gmail did not advertise its required stable identity and label extension.")
	}
	session.condstore = session.capabilities["CONDSTORE"] || session.capabilities["QRESYNC"]
	session.idle = session.capabilities["IDLE"]
	if !session.capabilities["QRESYNC"] || !session.capabilities["ENABLE"] {
		return nil
	}
	status, _, err = session.command("ENABLE QRESYNC")
	// Gmail's unsolicited QRESYNC FETCH responses are not required to include
	// X-GM-MSGID, so they cannot safely address the canonical one-message/many-
	// label projection. Gmail retains CONDSTORE plus IDLE and fetches bounded
	// metadata rows that explicitly include X-GM-MSGID and X-GM-LABELS.
	session.qresync = status == "OK" && session.provider != accounts.ProviderGmail
	return err
}

func capabilitySet(lines []string) map[string]bool {
	capabilities := map[string]bool{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.EqualFold(fields[0], "*") || !strings.EqualFold(fields[1], "CAPABILITY") {
			continue
		}
		for _, capability := range fields[2:] {
			capabilities[strings.ToUpper(capability)] = true
		}
	}
	return capabilities
}

func (session *imapSession) close() {
	if session == nil || session.connection == nil {
		return
	}
	_ = session.connection.SetDeadline(time.Now().Add(5 * time.Second))
	_, _, _ = session.command("LOGOUT")
	_ = session.connection.Close()
}

func (session *imapSession) command(command string) (string, []string, error) {
	session.tag++
	tag := fmt.Sprintf("c%04d", session.tag)
	if err := writeProtocolLine(session.connection, tag+" "+command); err != nil {
		return "", nil, safeConnectorError(accountsync.FailureTransient, "imap_write_failed", "The IMAP connection closed while sending a command.")
	}
	lines := make([]string, 0, 8)
	for count := 0; count < maximumReplyLines*4; count++ {
		line, err := readProtocolLine(session.reader)
		if err != nil {
			return "", nil, safeConnectorError(accountsync.FailureTransient, "imap_read_failed", "The IMAP connection closed while reading a response.")
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], tag) {
			lines = append(lines, line)
			return strings.ToUpper(fields[1]), lines, nil
		}
		lines = append(lines, line)
	}
	return "", nil, safeConnectorError(accountsync.FailurePermanent, "imap_response_limit", "The IMAP server exceeded the response-line limit.")
}

func (session *imapSession) waitForChange(ctx context.Context, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := session.connection.SetReadDeadline(deadline); err != nil {
		return false, safeConnectorError(accountsync.FailureTransient, "imap_idle_deadline_failed", "CADUCEUS could not bound the IMAP notification connection.")
	}
	defer session.connection.SetReadDeadline(time.Time{})
	stopCancellation := context.AfterFunc(ctx, func() { _ = session.connection.SetReadDeadline(time.Now()) })
	defer stopCancellation()

	session.tag++
	tag := fmt.Sprintf("c%04d", session.tag)
	if err := writeProtocolLine(session.connection, tag+" IDLE"); err != nil {
		return false, safeConnectorError(accountsync.FailureTransient, "imap_idle_write_failed", "The IMAP notification connection closed while starting IDLE.")
	}
	continuation, err := readProtocolLine(session.reader)
	if err != nil {
		return false, idleReadError(ctx)
	}
	if !strings.HasPrefix(strings.TrimSpace(continuation), "+") {
		return false, accountsync.ErrWatchUnsupported
	}
	return session.readIdleChanges(ctx, tag)
}

func (session *imapSession) readIdleChanges(ctx context.Context, tag string) (bool, error) {
	for count := 0; count < maximumReplyLines*4; count++ {
		result, err := session.readIdleLine(ctx, tag)
		if err != nil {
			return false, err
		}
		switch result {
		case idleChanged:
			return true, session.finishIdle(ctx, tag)
		case idleCompleted:
			return false, nil
		}
	}
	return false, safeConnectorError(accountsync.FailurePermanent, "imap_idle_response_limit", "The IMAP server exceeded the IDLE response-line limit.")
}

type idleLineResult uint8

const (
	idleContinues idleLineResult = iota
	idleChanged
	idleCompleted
)

func (session *imapSession) readIdleLine(ctx context.Context, tag string) (idleLineResult, error) {
	line, err := readProtocolLine(session.reader)
	if err != nil {
		return session.handleIdleReadError(ctx, tag, err)
	}
	return classifyIdleLine(tag, line)
}

func (session *imapSession) handleIdleReadError(ctx context.Context, tag string, err error) (idleLineResult, error) {
	if isTimeout(err) {
		return idleCompleted, session.finishIdle(ctx, tag)
	}
	return idleCompleted, idleReadError(ctx)
}

func classifyIdleLine(tag, line string) (idleLineResult, error) {
	if idleChangeLine(line) {
		return idleChanged, nil
	}
	fields := strings.Fields(line)
	if idleTagged(fields, tag) {
		return idleCompleted, nil
	}
	if idleBye(fields) {
		return idleCompleted, safeConnectorError(accountsync.FailureTransient, "imap_idle_closed", "The IMAP server closed the notification connection.")
	}
	return idleContinues, nil
}

func idleTagged(fields []string, tag string) bool {
	return len(fields) >= 2 && strings.EqualFold(fields[0], tag)
}

func idleBye(fields []string) bool {
	return len(fields) >= 2 && fields[0] == "*" && strings.EqualFold(fields[1], "BYE")
}

func (session *imapSession) finishIdle(ctx context.Context, tag string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = session.connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := writeProtocolLine(session.connection, "DONE"); err != nil {
		return safeConnectorError(accountsync.FailureTransient, "imap_idle_done_failed", "The IMAP notification connection closed while ending IDLE.")
	}
	for count := 0; count < maximumReplyLines; count++ {
		line, err := readProtocolLine(session.reader)
		if err != nil {
			return idleReadError(ctx)
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], tag) {
			if strings.EqualFold(fields[1], "OK") {
				return nil
			}
			return safeConnectorError(accountsync.FailureTransient, "imap_idle_rejected", "The IMAP server rejected the notification wait.")
		}
	}
	return safeConnectorError(accountsync.FailurePermanent, "imap_idle_response_limit", "The IMAP server exceeded the IDLE completion limit.")
}

func idleChangeLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "*" {
		return false
	}
	for _, field := range fields[1:] {
		switch strings.ToUpper(strings.Trim(field, "()")) {
		case "EXISTS", "RECENT", "EXPUNGE", "FETCH":
			return true
		}
	}
	return false
}

func idleReadError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return safeConnectorError(accountsync.FailureTransient, "imap_idle_read_failed", "The IMAP notification connection closed while waiting for changes.")
}

func isTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func (session *imapSession) listFolders() ([]accountsync.Folder, error) {
	status, lines, err := session.command(`LIST "" "*"`)
	if err != nil {
		return nil, err
	}
	if status != "OK" {
		return nil, safeConnectorError(accountsync.FailurePermanent, "imap_list_failed", "The IMAP server refused to list folders.")
	}
	result := []accountsync.Folder{}
	for _, line := range lines {
		folder, ok := parseListFolder(line)
		if ok {
			result = append(result, folder)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].StableID < result[right].StableID })
	return result, nil
}

type selectedFolder struct {
	uidValidity   uint64
	uidNext       uint64
	highestModSeq uint64
	updates       []accountsync.MessageUpdate
	vanishedUIDs  []uint64
	qresync       bool
}

func (session *imapSession) selectFolder(folder string) (selectedFolder, error) {
	command := "SELECT " + imapQuote(folder)
	if session.condstore {
		command += " (CONDSTORE)"
	}
	status, lines, err := session.command(command)
	if err != nil {
		return selectedFolder{}, err
	}
	if status != "OK" {
		return selectedFolder{}, safeConnectorError(accountsync.FailurePermanent, "imap_select_failed", "The IMAP server refused to open a folder.")
	}
	return parseSelectedFolder(lines)
}

func (session *imapSession) selectFolderChanges(folder string, state imapFolderCursor) (selectedFolder, error) {
	if !canQResync(session, state) {
		return session.selectFolder(folder)
	}
	lines, accepted, err := session.qresyncSelect(folder, state)
	if err != nil {
		return selectedFolder{}, err
	}
	if !accepted {
		return session.selectFolder(folder)
	}
	result, err := parseSelectedFolder(lines)
	if err != nil {
		return selectedFolder{}, err
	}
	return session.finishQResyncSelection(folder, state, lines, result)
}

func canQResync(session *imapSession, state imapFolderCursor) bool {
	return session.qresync && state.UIDValidity > 0 && state.HighestModSeq > 0
}

func (session *imapSession) qresyncSelect(folder string, state imapFolderCursor) ([]string, bool, error) {
	command := fmt.Sprintf("SELECT %s (QRESYNC (%d %d))", imapQuote(folder), state.UIDValidity, state.HighestModSeq)
	status, lines, err := session.command(command)
	if err != nil {
		return nil, false, err
	}
	if status != "OK" {
		session.qresync = false
		return nil, false, nil
	}
	return lines, true, nil
}

func (session *imapSession) finishQResyncSelection(folder string, state imapFolderCursor, lines []string, result selectedFolder) (selectedFolder, error) {
	if modSeqRegressed(result, state) {
		return selectedFolder{}, safeConnectorError(accountsync.FailurePermanent, "imap_modseq_regressed", "The IMAP server returned a regressed mailbox modification sequence.")
	}
	result.qresync = result.uidValidity == state.UIDValidity && result.highestModSeq > 0
	if !result.qresync {
		session.qresync = false
		return result, nil
	}
	if !collectQResyncChanges(folder, state, lines, &result) {
		session.qresync = false
		return result, nil
	}
	normalizeQResyncChanges(&result)
	return result, nil
}

func collectQResyncChanges(folder string, state imapFolderCursor, lines []string, result *selectedFolder) bool {
	for _, line := range lines {
		if match := imapVanishedPattern.FindStringSubmatch(line); len(match) == 2 {
			uids, parseErr := parseUIDSet(match[1], accountsync.MaxBatchDeletes-len(result.vanishedUIDs))
			if parseErr != nil {
				clearQResyncChanges(result)
				return false
			}
			result.vanishedUIDs = append(result.vanishedUIDs, uids...)
			continue
		}
		uid := parseUintMatch(imapUIDPattern, line)
		if uid == 0 || uid > state.LastUID || parseUintMatch(imapModSeqPattern, line) <= state.HighestModSeq {
			continue
		}
		flags := parseFlags(line)
		stableID := fmt.Sprintf("%s:%d:%d", folder, state.UIDValidity, uid)
		result.updates = append(result.updates, accountsync.MessageUpdate{StableID: stableID, Revision: messageRevision(state.UIDValidity, uid, flags), Flags: flags})
		if len(result.updates) > accountsync.MaxBatchUpdates {
			clearQResyncChanges(result)
			return false
		}
	}
	return true
}

func clearQResyncChanges(result *selectedFolder) {
	result.qresync = false
	result.vanishedUIDs = nil
	result.updates = nil
}

func normalizeQResyncChanges(result *selectedFolder) {
	vanished := make(map[uint64]bool, len(result.vanishedUIDs))
	for _, uid := range result.vanishedUIDs {
		vanished[uid] = true
	}
	result.vanishedUIDs = result.vanishedUIDs[:0]
	for uid := range vanished {
		result.vanishedUIDs = append(result.vanishedUIDs, uid)
	}
	sort.Slice(result.vanishedUIDs, func(i, j int) bool { return result.vanishedUIDs[i] < result.vanishedUIDs[j] })
	updates := make(map[string]accountsync.MessageUpdate, len(result.updates))
	for _, update := range result.updates {
		_, _, uid, _ := parseMessageStableID(update.StableID)
		if !vanished[uid] {
			updates[update.StableID] = update
		}
	}
	result.updates = result.updates[:0]
	for _, update := range updates {
		result.updates = append(result.updates, update)
	}
	sort.Slice(result.updates, func(i, j int) bool { return result.updates[i].StableID < result.updates[j].StableID })
}

func parseSelectedFolder(lines []string) (selectedFolder, error) {
	var result selectedFolder
	for _, line := range lines {
		if match := imapUIDValidityPattern.FindStringSubmatch(line); len(match) == 2 {
			result.uidValidity, _ = strconv.ParseUint(match[1], 10, 64)
		}
		if match := imapUIDNextPattern.FindStringSubmatch(line); len(match) == 2 {
			result.uidNext, _ = strconv.ParseUint(match[1], 10, 64)
		}
		if match := imapHighestModSeq.FindStringSubmatch(line); len(match) == 2 {
			result.highestModSeq, _ = strconv.ParseUint(match[1], 10, 64)
		}
	}
	if result.uidValidity == 0 {
		return selectedFolder{}, safeConnectorError(accountsync.FailurePermanent, "imap_uidvalidity_missing", "The IMAP folder did not provide UIDVALIDITY.")
	}
	return result, nil
}

func parseUIDSet(value string, limit int) ([]uint64, error) {
	if limit < 1 {
		return nil, errors.New("IMAP UID set exceeds the synchronization bound")
	}
	seen := map[uint64]bool{}
	for _, part := range strings.Split(strings.TrimSpace(value), ",") {
		bounds := strings.Split(part, ":")
		if len(bounds) < 1 || len(bounds) > 2 {
			return nil, errors.New("invalid IMAP UID set")
		}
		start, err := strconv.ParseUint(bounds[0], 10, 64)
		if err != nil || start == 0 {
			return nil, errors.New("invalid IMAP UID set")
		}
		end := start
		if len(bounds) == 2 {
			end, err = strconv.ParseUint(bounds[1], 10, 64)
			if err != nil || end < start {
				return nil, errors.New("invalid IMAP UID set")
			}
		}
		if end-start+1 > uint64(limit-len(seen)) {
			return nil, errors.New("IMAP UID set exceeds the synchronization bound")
		}
		for uid := start; uid <= end; uid++ {
			seen[uid] = true
			if uid == ^uint64(0) {
				break
			}
		}
	}
	result := make([]uint64, 0, len(seen))
	for uid := range seen {
		result = append(result, uid)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func (session *imapSession) searchUIDs(start uint64) ([]uint64, error) {
	if start == 0 {
		start = 1
	}
	status, lines, err := session.command(fmt.Sprintf("UID SEARCH UID %d:*", start))
	if err != nil {
		return nil, err
	}
	if status != "OK" {
		return nil, safeConnectorError(accountsync.FailurePermanent, "imap_search_failed", "The IMAP server refused an incremental UID search.")
	}
	result := []uint64{}
	for _, line := range lines {
		if !strings.HasPrefix(strings.ToUpper(line), "* SEARCH") {
			continue
		}
		for _, field := range strings.Fields(line)[2:] {
			uid, parseErr := strconv.ParseUint(field, 10, 64)
			if parseErr == nil && uid >= start {
				result = append(result, uid)
			}
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result, nil
}

func (session *imapSession) fetchMessage(folder string, uidValidity, uid uint64, maxBytes int) (accountsync.Message, error) {
	metadata := "UID FLAGS RFC822.SIZE INTERNALDATE"
	if session.provider == accounts.ProviderGmail {
		metadata += " X-GM-MSGID X-GM-LABELS"
	}
	line, raw, err := session.fetchRawMessage(uid, metadata, maxBytes)
	if err != nil {
		return accountsync.Message{}, err
	}
	return session.projectFetchedMessage(folder, uidValidity, uid, line, raw)
}

func (session *imapSession) fetchRawMessage(uid uint64, metadata string, maxBytes int) (string, []byte, error) {
	session.tag++
	tag := fmt.Sprintf("c%04d", session.tag)
	if err := writeProtocolLine(session.connection, fmt.Sprintf("%s UID FETCH %d (%s BODY.PEEK[])", tag, uid, metadata)); err != nil {
		return "", nil, err
	}
	line, err := readProtocolLine(session.reader)
	if err != nil {
		return "", nil, safeConnectorError(accountsync.FailureTransient, "imap_fetch_failed", "The IMAP message response was interrupted.")
	}
	literal := imapLiteralPattern.FindStringSubmatch(line)
	if len(literal) != 2 {
		return "", nil, safeConnectorError(accountsync.FailurePermanent, "imap_literal_missing", "The IMAP server omitted the requested raw message.")
	}
	size, err := strconv.Atoi(literal[1])
	if err != nil || size <= 0 || size > maxBytes {
		return "", nil, safeConnectorError(accountsync.FailurePermanent, "message_size_limit", "An IMAP message exceeds this server's import limit.")
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(session.reader, raw); err != nil {
		return "", nil, safeConnectorError(accountsync.FailureTransient, "imap_fetch_failed", "The IMAP message response was interrupted.")
	}
	status, err := session.readFetchStatus(tag)
	if err != nil {
		return "", nil, err
	}
	if status != "OK" {
		return "", nil, safeConnectorError(accountsync.FailurePermanent, "imap_fetch_rejected", "The IMAP server refused a message fetch.")
	}
	return line, raw, nil
}

func (session *imapSession) readFetchStatus(tag string) (string, error) {
	for count := 0; count < maximumReplyLines; count++ {
		continuation, err := readProtocolLine(session.reader)
		if err != nil {
			return "", safeConnectorError(accountsync.FailureTransient, "imap_fetch_failed", "The IMAP message response was interrupted.")
		}
		fields := strings.Fields(continuation)
		if len(fields) >= 2 && strings.EqualFold(fields[0], tag) {
			return strings.ToUpper(fields[1]), nil
		}
	}
	return "", safeConnectorError(accountsync.FailurePermanent, "imap_response_limit", "The IMAP server exceeded the response-line limit.")
}

func (session *imapSession) projectFetchedMessage(folder string, uidValidity, uid uint64, line string, raw []byte) (accountsync.Message, error) {
	returnedUID := parseUintMatch(imapUIDPattern, line)
	if returnedUID != uid {
		return accountsync.Message{}, safeConnectorError(accountsync.FailurePermanent, "imap_uid_mismatch", "The IMAP server returned an unexpected message identity.")
	}
	receivedAt := time.Now().UTC()
	if match := imapDatePattern.FindStringSubmatch(line); len(match) == 2 {
		if parsed, parseErr := time.Parse("02-Jan-2006 15:04:05 -0700", match[1]); parseErr == nil {
			receivedAt = parsed.UTC()
		}
	}
	index := indexRFC822(raw)
	flags := parseFlags(line)
	stableID, remoteStableID := imapStableID(folder, uidValidity, uid), ""
	folderStableIDs := []string(nil)
	revision := messageRevision(uidValidity, uid, flags)
	if session.provider == accounts.ProviderGmail {
		gmailID := parseUintMatch(imapGmailMessageID, line)
		if gmailID == 0 {
			return accountsync.Message{}, safeConnectorError(accountsync.FailurePermanent, "gmail_message_identity_missing", "Gmail omitted its stable message identity.")
		}
		labels := parseGmailLabels(line)
		stableID, remoteStableID = gmailStableID(gmailID), stableID
		folderStableIDs = session.gmailFolderMemberships(folder, labels)
		revision = gmailMessageRevision(gmailID, flags, folderStableIDs)
	}
	return accountsync.Message{StableID: stableID, RemoteStableID: remoteStableID, Revision: revision, FolderStableID: folder, FolderStableIDs: folderStableIDs,
		RFCMessageID: index.RFCMessageID, InReplyTo: index.InReplyTo, References: index.References, Subject: index.Subject, Sender: index.Sender, To: index.To, CC: index.CC, BCC: index.BCC,
		Snippet: index.Snippet, AttachmentNames: index.AttachmentNames, ReceivedAt: receivedAt, Flags: flags, Size: uint64(len(raw)), RawRFC822: raw}, nil
}

func imapStableID(folder string, uidValidity, uid uint64) string {
	return fmt.Sprintf("%s:%d:%d", folder, uidValidity, uid)
}

func (session *imapSession) fetchMessageUpdates(folder string, uidValidity, start, end uint64) ([]accountsync.MessageUpdate, error) {
	status, lines, err := session.command(fmt.Sprintf("UID FETCH %d:%d (%s)", start, end, session.metadataFields(false)))
	if err != nil {
		return nil, err
	}
	if status != "OK" {
		return nil, safeConnectorError(accountsync.FailurePermanent, "imap_delta_fetch_failed", "The IMAP server refused a bounded metadata refresh.")
	}
	updates := []accountsync.MessageUpdate{}
	for _, line := range lines {
		uid := parseUintMatch(imapUIDPattern, line)
		if uid < start || uid > end {
			continue
		}
		update, updateErr := session.messageUpdate(folder, uidValidity, uid, line)
		if updateErr != nil {
			return nil, updateErr
		}
		updates = append(updates, update)
	}
	sort.Slice(updates, func(left, right int) bool { return updates[left].StableID < updates[right].StableID })
	return updates, nil
}

func (session *imapSession) fetchMessageUpdatesSince(folder string, uidValidity, start, end, modSeq uint64) ([]accountsync.MessageUpdate, error) {
	status, lines, err := session.command(fmt.Sprintf("UID FETCH %d:%d (%s) (CHANGEDSINCE %d)", start, end, session.metadataFields(true), modSeq))
	if err != nil {
		return nil, err
	}
	if status != "OK" {
		return nil, safeConnectorError(accountsync.FailurePermanent, "imap_condstore_fetch_failed", "The IMAP server refused a bounded CONDSTORE metadata refresh.")
	}
	updates := []accountsync.MessageUpdate{}
	for _, line := range lines {
		uid := parseUintMatch(imapUIDPattern, line)
		if uid < start || uid > end || parseUintMatch(imapModSeqPattern, line) <= modSeq {
			continue
		}
		update, updateErr := session.messageUpdate(folder, uidValidity, uid, line)
		if updateErr != nil {
			return nil, updateErr
		}
		updates = append(updates, update)
	}
	sort.Slice(updates, func(left, right int) bool { return updates[left].StableID < updates[right].StableID })
	return updates, nil
}

func (session *imapSession) searchUIDRange(folder string, uidValidity, start, end uint64) ([]string, error) {
	status, lines, err := session.command(fmt.Sprintf("UID SEARCH UID %d:%d", start, end))
	if err != nil {
		return nil, err
	}
	if status != "OK" {
		return nil, safeConnectorError(accountsync.FailurePermanent, "imap_search_failed", "The IMAP server refused a bounded UID reconciliation search.")
	}
	uids := []uint64{}
	for _, line := range lines {
		if !strings.HasPrefix(strings.ToUpper(line), "* SEARCH") {
			continue
		}
		for _, field := range strings.Fields(line)[2:] {
			uid, parseErr := strconv.ParseUint(field, 10, 64)
			if parseErr == nil && uid >= start && uid <= end {
				uids = append(uids, uid)
			}
		}
	}
	sort.Slice(uids, func(left, right int) bool { return uids[left] < uids[right] })
	result := make([]string, 0, len(uids))
	for _, uid := range uids {
		result = append(result, fmt.Sprintf("%s:%d:%d", folder, uidValidity, uid))
	}
	return result, nil
}

func parseFlags(line string) []string {
	flags := []string{}
	if match := imapFlagsPattern.FindStringSubmatch(line); len(match) == 2 {
		flags = strings.Fields(match[1])
	}
	sort.Strings(flags)
	return flags
}

func parseGmailLabels(line string) []string {
	upper := strings.ToUpper(line)
	marker := strings.Index(upper, "X-GM-LABELS")
	if marker < 0 {
		return nil
	}
	start := strings.Index(line[marker+len("X-GM-LABELS"):], "(")
	if start < 0 {
		return nil
	}
	start += marker + len("X-GM-LABELS") + 1
	quoted, escaped := false, false
	for index := start; index < len(line); index++ {
		switch {
		case escaped:
			escaped = false
		case quoted && line[index] == '\\':
			escaped = true
		case line[index] == '"':
			quoted = !quoted
		case !quoted && line[index] == ')':
			return parseIMAPListValues(line[start:index])
		}
	}
	return nil
}

func parseIMAPListValues(value string) []string {
	result := []string{}
	for index := 0; index < len(value); {
		item, next, ok := nextIMAPListValue(value, index)
		if !ok {
			break
		}
		index = next
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func nextIMAPListValue(value string, index int) (string, int, bool) {
	for index < len(value) && (value[index] == ' ' || value[index] == '\t') {
		index++
	}
	if index >= len(value) {
		return "", index, false
	}
	if value[index] != '"' {
		end := strings.IndexAny(value[index:], " \t")
		if end < 0 {
			return value[index:], len(value), true
		}
		return value[index : index+end], index + end, true
	}
	item, remaining, ok := parseIMAPToken(value[index:])
	return item, len(value) - len(remaining), ok
}

func (session *imapSession) gmailFolderMemberships(current string, labels []string) []string {
	seen := map[string]bool{current: true}
	for _, label := range labels {
		if folder := session.gmailFolders[strings.ToLower(label)]; folder != "" {
			seen[folder] = true
		}
	}
	result := make([]string, 0, len(seen))
	for folder := range seen {
		result = append(result, folder)
	}
	sort.Strings(result)
	return result
}

func messageRevision(uidValidity, uid uint64, flags []string) string {
	revisionHash := sha256.Sum256([]byte(strings.Join(flags, "\x00")))
	return fmt.Sprintf("%d:%d:%s", uidValidity, uid, hex.EncodeToString(revisionHash[:8]))
}

func (session *imapSession) metadataFields(modSeq bool) string {
	fields := "UID FLAGS"
	if modSeq {
		fields += " MODSEQ"
	}
	if session.provider == accounts.ProviderGmail {
		fields += " X-GM-MSGID X-GM-LABELS"
	}
	return fields
}

func (session *imapSession) messageUpdate(folder string, uidValidity, uid uint64, line string) (accountsync.MessageUpdate, error) {
	flags := parseFlags(line)
	stableID := fmt.Sprintf("%s:%d:%d", folder, uidValidity, uid)
	remoteStableID := ""
	memberships := []string(nil)
	revision := messageRevision(uidValidity, uid, flags)
	if session.provider == accounts.ProviderGmail {
		gmailID := parseUintMatch(imapGmailMessageID, line)
		if gmailID == 0 {
			return accountsync.MessageUpdate{}, safeConnectorError(accountsync.FailurePermanent, "gmail_message_identity_missing", "Gmail omitted its stable message identity.")
		}
		memberships = session.gmailFolderMemberships(folder, parseGmailLabels(line))
		stableID, remoteStableID, revision = gmailStableID(gmailID), stableID, gmailMessageRevision(gmailID, flags, memberships)
	}
	return accountsync.MessageUpdate{StableID: stableID, RemoteStableID: remoteStableID, Revision: revision, FolderStableIDs: memberships, Flags: flags}, nil
}

func gmailStableID(messageID uint64) string {
	return "gmail:" + strconv.FormatUint(messageID, 10)
}

func gmailMessageRevision(messageID uint64, flags, memberships []string) string {
	material := append(append([]string{}, flags...), memberships...)
	hash := sha256.Sum256([]byte(strings.Join(material, "\x00")))
	return fmt.Sprintf("gmail:%d:%s", messageID, hex.EncodeToString(hash[:8]))
}

func (session *imapSession) apply(mutation accountsync.Mutation, uid uint64) (accountsync.MutationResult, error) {
	command := ""
	switch mutation.Kind {
	case accountsync.MutationSetRead:
		operator := "+FLAGS.SILENT"
		if !mutation.Value {
			operator = "-FLAGS.SILENT"
		}
		command = fmt.Sprintf("UID STORE %d %s (\\Seen)", uid, operator)
	case accountsync.MutationSetStarred:
		operator := "+FLAGS.SILENT"
		if !mutation.Value {
			operator = "-FLAGS.SILENT"
		}
		command = fmt.Sprintf("UID STORE %d %s (\\Flagged)", uid, operator)
	case accountsync.MutationMove:
		command = fmt.Sprintf("UID MOVE %d %s", uid, imapQuote(mutation.TargetFolderID))
	case accountsync.MutationDelete:
		return session.deleteUID(uid)
	default:
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied}, nil
	}
	status, lines, err := session.command(command)
	if err != nil {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown}, nil
	}
	if status == "OK" {
		if mutation.Kind == accountsync.MutationMove {
			for _, line := range lines {
				match := imapCopyUIDPattern.FindStringSubmatch(line)
				if len(match) != 4 || match[2] != strconv.FormatUint(uid, 10) {
					continue
				}
				uidValidity, validityErr := strconv.ParseUint(match[1], 10, 64)
				destinationUID, uidErr := strconv.ParseUint(match[3], 10, 64)
				if validityErr == nil && uidErr == nil && uidValidity > 0 && destinationUID > 0 {
					stableID := fmt.Sprintf("%s:%d:%d", mutation.TargetFolderID, uidValidity, destinationUID)
					return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied, ResultStableIDs: []string{stableID}}, nil
				}
			}
			return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "imap_copyuid_missing"}, nil
		}
		return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied}, nil
}

// deleteUID expunges only the selected UID. A mailbox-wide EXPUNGE could
// permanently remove messages another client marked deleted, so a server
// without UIDPLUS is failed closed after rolling back our own flag change.
func (session *imapSession) deleteUID(uid uint64) (accountsync.MutationResult, error) {
	status, _, err := session.command(fmt.Sprintf("UID STORE %d +FLAGS.SILENT (\\Deleted)", uid))
	if err != nil {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown}, nil
	}
	if status != "OK" {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "imap_delete_rejected"}, nil
	}
	status, _, err = session.command(fmt.Sprintf("UID EXPUNGE %d", uid))
	if err != nil {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown}, nil
	}
	if status == "OK" {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied}, nil
	}
	rollbackStatus, _, rollbackErr := session.command(fmt.Sprintf("UID STORE %d -FLAGS.SILENT (\\Deleted)", uid))
	if rollbackErr != nil || rollbackStatus != "OK" {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown}, nil
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "imap_uid_expunge_unsupported"}, nil
}

func (session *imapSession) reconcile(mutation accountsync.Mutation, uid uint64) (bool, error) {
	if mutation.Kind == accountsync.MutationDelete {
		uids, err := session.searchUIDs(uid)
		if err != nil {
			return false, err
		}
		for _, candidate := range uids {
			if candidate == uid {
				return false, nil
			}
		}
		return true, nil
	}
	status, lines, err := session.command(fmt.Sprintf("UID FETCH %d (FLAGS)", uid))
	if err != nil {
		return false, err
	}
	if status != "OK" {
		return false, nil
	}
	flags := ""
	for _, line := range lines {
		if match := imapFlagsPattern.FindStringSubmatch(line); len(match) == 2 {
			flags = " " + strings.ToLower(match[1]) + " "
		}
	}
	wantFlag := " \\seen "
	if mutation.Kind == accountsync.MutationSetStarred {
		wantFlag = " \\flagged "
	}
	present := strings.Contains(flags, wantFlag)
	return present == mutation.Value, nil
}

func (session *imapSession) reconcileMove(mutation accountsync.Mutation) (accountsync.MutationResult, error) {
	if len(mutation.ResultStableIDs) == 0 {
		allSourcesPresent := true
		for _, stableID := range mutation.MessageStableIDs {
			present, valid, err := session.messageIdentityPresent(stableID)
			if err != nil {
				return accountsync.MutationResult{}, err
			}
			if !valid || !present {
				allSourcesPresent = false
			}
		}
		if allSourcesPresent {
			return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "mutation_not_applied"}, nil
		}
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "imap_copyuid_missing"}, nil
	}
	if err := accountsync.ValidateMutationResults(mutation); err != nil {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "imap_copyuid_invalid"}, nil
	}
	allApplied, allNotApplied := true, true
	for index, sourceID := range mutation.MessageStableIDs {
		destinationID := mutation.ResultStableIDs[index]
		destinationFolder, _, _, ok := parseMessageStableID(destinationID)
		if !ok || destinationFolder != mutation.TargetFolderID {
			return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "imap_copyuid_invalid"}, nil
		}
		sourcePresent, sourceValid, err := session.messageIdentityPresent(sourceID)
		if err != nil {
			return accountsync.MutationResult{}, err
		}
		destinationPresent, destinationValid, err := session.messageIdentityPresent(destinationID)
		if err != nil {
			return accountsync.MutationResult{}, err
		}
		applied := sourceValid && destinationValid && !sourcePresent && destinationPresent
		notApplied := sourceValid && destinationValid && sourcePresent && !destinationPresent
		allApplied = allApplied && applied
		allNotApplied = allNotApplied && notApplied
	}
	if allApplied {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeApplied, ResultStableIDs: append([]string(nil), mutation.ResultStableIDs...)}, nil
	}
	if allNotApplied {
		return accountsync.MutationResult{Outcome: accountsync.OutcomeNotApplied, Code: "mutation_not_applied"}, nil
	}
	return accountsync.MutationResult{Outcome: accountsync.OutcomeUnknown, Code: "imap_move_state_ambiguous"}, nil
}

func (session *imapSession) messageIdentityPresent(stableID string) (bool, bool, error) {
	folder, uidValidity, uid, ok := parseMessageStableID(stableID)
	if !ok {
		return false, false, nil
	}
	selected, err := session.selectFolder(folder)
	if err != nil {
		return false, false, err
	}
	if selected.uidValidity != uidValidity {
		return false, false, nil
	}
	uids, err := session.searchUIDs(uid)
	if err != nil {
		return false, true, err
	}
	for _, candidate := range uids {
		if candidate == uid {
			return true, true, nil
		}
	}
	return false, true, nil
}

func parseListFolder(line string) (accountsync.Folder, bool) {
	if !strings.HasPrefix(strings.ToUpper(line), "* LIST ") {
		return accountsync.Folder{}, false
	}
	attributeStart, attributeEnd := strings.Index(line, "("), strings.Index(line, ")")
	if attributeStart < 0 || attributeEnd <= attributeStart {
		return accountsync.Folder{}, false
	}
	delimiter, wireName, ok := parseListWireFields(line[attributeEnd+1:])
	if !ok {
		return accountsync.Folder{}, false
	}
	if strings.EqualFold(delimiter, "NIL") {
		delimiter = ""
	}
	name, err := imapwire.DecodeMailbox(wireName)
	if err != nil || !validMailboxName(wireName, name) {
		return accountsync.Folder{}, false
	}
	attributes := strings.Fields(line[attributeStart+1 : attributeEnd])
	return accountsync.Folder{StableID: wireName, Name: name, Role: folderRole(name, attributes), Delimiter: delimiter, Attributes: attributes}, true
}

func parseListWireFields(value string) (string, string, bool) {
	delimiter, tail, delimiterOK := parseIMAPToken(value)
	wireName, remaining, mailboxOK := parseIMAPToken(tail)
	ok := delimiterOK && mailboxOK && strings.TrimSpace(remaining) == "" && !strings.EqualFold(wireName, "NIL")
	return delimiter, wireName, ok
}

func validMailboxName(wireName, name string) bool {
	return wireName != "" && len(wireName) <= 1024 && len(name) <= 1024 && !strings.ContainsAny(name, "\x00\r\n")
}

func parseIMAPToken(value string) (string, string, bool) {
	value = strings.TrimLeft(value, " \t")
	if value == "" {
		return "", "", false
	}
	if value[0] != '"' {
		end := strings.IndexAny(value, " \t")
		if end < 0 {
			return value, "", !strings.ContainsAny(value, "(){}\r\n")
		}
		return value[:end], value[end:], !strings.ContainsAny(value[:end], "(){}\r\n")
	}
	var builder strings.Builder
	for index := 1; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			index++
			builder.WriteByte(value[index])
			continue
		}
		if value[index] == '"' {
			return builder.String(), value[index+1:], true
		}
		builder.WriteByte(value[index])
	}
	return "", "", false
}

func quotedIMAPValues(value string) []string {
	result := []string{}
	for index := 0; index < len(value); index++ {
		if value[index] != '"' {
			continue
		}
		var builder strings.Builder
		for index++; index < len(value); index++ {
			if value[index] == '\\' && index+1 < len(value) {
				index++
				builder.WriteByte(value[index])
				continue
			}
			if value[index] == '"' {
				break
			}
			builder.WriteByte(value[index])
		}
		result = append(result, builder.String())
	}
	return result
}

func folderRole(name string, attributes []string) string {
	joined := strings.ToLower(strings.Join(attributes, " "))
	for marker, role := range map[string]string{"\\inbox": "inbox", "\\sent": "sent", "\\drafts": "drafts", "\\junk": "spam", "\\trash": "trash", "\\archive": "archive", "\\all": "all", "\\important": "important"} {
		if strings.Contains(joined, marker) {
			return role
		}
	}
	lower := strings.ToLower(name)
	if lower == "inbox" {
		return "inbox"
	}
	return ""
}

func parseMessageStableID(value string) (string, uint64, uint64, bool) {
	parts := strings.Split(value, ":")
	if len(parts) < 3 {
		return "", 0, 0, false
	}
	uidValidity, errA := strconv.ParseUint(parts[len(parts)-2], 10, 64)
	uid, errB := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	folder := strings.Join(parts[:len(parts)-2], ":")
	return folder, uidValidity, uid, errA == nil && errB == nil && folder != ""
}

func parseUintMatch(pattern *regexp.Regexp, value string) uint64 {
	match := pattern.FindStringSubmatch(value)
	if len(match) != 2 {
		return 0
	}
	result, _ := strconv.ParseUint(match[1], 10, 64)
	return result
}

func safeConnectorError(kind accountsync.FailureKind, code, message string) error {
	return accountsync.ConnectorError{Kind: kind, Code: code, Message: message}
}
