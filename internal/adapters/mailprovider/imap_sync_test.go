package mailprovider

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestIMAPConnectorOpensMicrosoftSharedMailboxWithDelegatedXOAUTH2(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		line, _ := reader.ReadString('\n')
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != "AUTHENTICATE" || fields[2] != "XOAUTH2" {
			t.Errorf("auth command = %q", line)
		} else if decoded, err := base64.StdEncoding.DecodeString(fields[3]); err != nil || string(decoded) != "user=support@example.com\x01auth=Bearer access-token\x01\x01" {
			t.Errorf("auth payload = %q, %v", decoded, err)
		}
		_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", fields[0])
		line, _ = reader.ReadString('\n')
		logout := strings.Fields(line)
		_, _ = fmt.Fprintf(connection, "* BYE\r\n%s OK logout\r\n", logout[0])
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, err := NewIMAPConnector(localPasswordValidator(t, time.Second), IMAPSyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	account := accounts.ExternalAccount{Provider: accounts.ProviderMicrosoft, MailboxAddress: "support@example.com", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	session, err := connector.open(context.Background(), account, accounts.Credentials{AccessToken: "access-token"})
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}
	session.close()
}

func TestIMAPConnectorImportsFolderAndRawMessageWithDurableCursor(t *testing.T) {
	raw := []byte("Message-ID: <fixture@example.net>\r\nFrom: Sender <sender@example.net>\r\nSubject: Imported message\r\n\r\nbody")
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
			switch {
			case strings.HasPrefix(command, "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case strings.HasPrefix(command, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\Inbox) \"/\" \"INBOX\"\r\n%s OK listed\r\n", tag)
			case strings.HasPrefix(command, "SELECT "):
				_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 10] next\r\n%s OK selected\r\n", tag)
			case strings.HasPrefix(command, "UID SEARCH "):
				_, _ = fmt.Fprintf(connection, "* SEARCH 9\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(command, "UID FETCH 9 "):
				_, _ = fmt.Fprintf(connection, "* 1 FETCH (UID 9 FLAGS (\\Seen) RFC822.SIZE %d INTERNALDATE \"21-Aug-2026 12:00:00 +0000\" BODY[] {%d}\r\n", len(raw), len(raw))
				_, _ = connection.Write(raw)
				_, _ = fmt.Fprintf(connection, ")\r\n%s OK fetched\r\n", tag)
			case strings.HasPrefix(command, "UID FETCH 1:2 "):
				_, _ = fmt.Fprintf(connection, "%s OK refreshed\r\n", tag)
			case strings.HasPrefix(command, "LOGOUT"):
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, err := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{MaxMessages: 10, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	account := accounts.ExternalAccount{
		ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net",
		Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}},
	}
	sink := &captureSink{}
	err = connector.Sync(context.Background(), accountsync.SyncRequest{
		Account: account, Credentials: accounts.Credentials{Username: "owner@example.net", Password: "secret"},
	}, sink)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(sink.batches) != 3 || len(sink.batches[0].Folders) != 1 || !sink.batches[0].FolderSnapshot || len(sink.batches[1].Messages) != 1 || len(sink.batches[2].Windows) != 1 {
		t.Fatalf("batches = %#v", sink.batches)
	}
	message := sink.batches[1].Messages[0]
	if message.StableID != "INBOX:77:9" || message.Subject != "Imported message" || string(message.RawRFC822) != string(raw) {
		t.Fatalf("message = %#v", message)
	}
	if !strings.Contains(sink.cursor, `"lastUid":9`) {
		t.Fatalf("cursor = %s", sink.cursor)
	}
}

func TestIMAPConnectorRefreshesFlagsAndDetectsRemoteExpungeInBoundedWindow(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
			switch {
			case strings.HasPrefix(command, "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case strings.HasPrefix(command, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\Inbox) \"/\" \"INBOX\"\r\n%s OK listed\r\n", tag)
			case strings.HasPrefix(command, "SELECT "):
				_, _ = fmt.Fprintf(connection, "* 2 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 4] next\r\n%s OK selected\r\n", tag)
			case strings.HasPrefix(command, "UID SEARCH "):
				_, _ = fmt.Fprintf(connection, "* SEARCH\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(command, "UID FETCH 1:3 "):
				_, _ = fmt.Fprintf(connection, "* 1 FETCH (UID 1 FLAGS (\\Seen))\r\n* 2 FETCH (UID 3 FLAGS (\\Flagged))\r\n%s OK refreshed\r\n", tag)
			case strings.HasPrefix(command, "LOGOUT"):
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, err := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{MaxMessages: 10, PageSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net",
		Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	checkpoint := accountsync.Checkpoint{Cursor: `{"folders":{"INBOX":{"uidValidity":77,"lastUid":3,"reconcileUid":1}}}`, Version: 4}
	sink := &captureSink{cursor: checkpoint.Cursor}
	if err := connector.Sync(context.Background(), accountsync.SyncRequest{Account: account, Credentials: accounts.Credentials{Password: "secret"}, Checkpoint: checkpoint}, sink); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(sink.batches) != 2 {
		t.Fatalf("batches = %#v", sink.batches)
	}
	delta := sink.batches[1]
	if len(delta.Updates) != 2 || len(delta.Windows) != 1 || len(delta.Windows[0].PresentIDs) != 2 || delta.Windows[0].StartUID != 1 || delta.Windows[0].EndUID != 3 {
		t.Fatalf("delta batch = %#v", delta)
	}
	if delta.Updates[0].StableID != "INBOX:77:1" || delta.Updates[0].Flags[0] != "\\Seen" || delta.Updates[1].StableID != "INBOX:77:3" {
		t.Fatalf("updates = %#v", delta.Updates)
	}
}

func TestIMAPConnectorUsesQRESYNCForFlagAndExpungeDeltas(t *testing.T) {
	commands := []string{}
	var commandsMu sync.Mutex
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
			commandsMu.Lock()
			commands = append(commands, command)
			commandsMu.Unlock()
			switch {
			case strings.HasPrefix(command, "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case command == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 ENABLE CONDSTORE QRESYNC IDLE\r\n%s OK capable\r\n", tag)
			case command == "ENABLE QRESYNC":
				_, _ = fmt.Fprintf(connection, "* ENABLED QRESYNC\r\n%s OK enabled\r\n", tag)
			case strings.HasPrefix(command, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\Inbox) \"/\" \"INBOX\"\r\n%s OK listed\r\n", tag)
			case command == `SELECT "INBOX" (CONDSTORE)`:
				_, _ = fmt.Fprintf(connection, "* 3 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 4] next\r\n* OK [HIGHESTMODSEQ 12] current\r\n%s OK selected\r\n", tag)
			case command == `SELECT "INBOX" (QRESYNC (77 10))`:
				_, _ = fmt.Fprintf(connection, "* 2 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 4] next\r\n* OK [HIGHESTMODSEQ 12] current\r\n* VANISHED (EARLIER) 2\r\n* 1 FETCH (UID 1 FLAGS (\\Seen) MODSEQ (11))\r\n%s OK selected\r\n", tag)
			case strings.HasPrefix(command, "UID SEARCH UID 4:"):
				_, _ = fmt.Fprintf(connection, "* SEARCH\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(command, "LOGOUT"):
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{MaxMessages: 10, PageSize: 3})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	checkpoint := accountsync.Checkpoint{Cursor: `{"folders":{"INBOX":{"uidValidity":77,"lastUid":3,"reconcileUid":1,"highestModSeq":10}}}`, Version: 4}
	sink := &captureSink{cursor: checkpoint.Cursor}
	if err := connector.Sync(context.Background(), accountsync.SyncRequest{Account: account, Credentials: accounts.Credentials{Password: "secret"}, Checkpoint: checkpoint}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 2 || len(sink.batches[1].Updates) != 1 || len(sink.batches[1].Deletions) != 1 || sink.batches[1].Deletions[0].StableID != "INBOX:77:2" || !strings.Contains(sink.cursor, `"highestModSeq":12`) {
		t.Fatalf("batches=%#v cursor=%s", sink.batches, sink.cursor)
	}
	commandsMu.Lock()
	joined := strings.Join(commands, "\n")
	commandsMu.Unlock()
	if !strings.Contains(joined, `ENABLE QRESYNC`) || !strings.Contains(joined, `SELECT "INBOX" (QRESYNC (77 10))`) || strings.Contains(joined, "CHANGEDSINCE") {
		t.Fatalf("commands=%q", joined)
	}
}

func TestGmailSyncUsesCanonicalMessageIdentityAndLabelMemberships(t *testing.T) {
	raw := []byte("Message-ID: <gmail-fixture@example.net>\r\nFrom: Sender <sender@example.net>\r\nSubject: Gmail labels\r\n\r\nbody")
	commands := []string{}
	var commandsMu sync.Mutex
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		selected := ""
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.TrimSpace(strings.TrimSuffix(line, "\r\n"))[len(fields[0])+1:]
			upper := strings.ToUpper(command)
			commandsMu.Lock()
			commands = append(commands, command)
			commandsMu.Unlock()
			switch {
			case strings.HasPrefix(upper, "AUTHENTICATE XOAUTH2 "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case upper == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 CONDSTORE IDLE X-GM-EXT-1\r\n%s OK capable\r\n", tag)
			case strings.HasPrefix(upper, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\Inbox) \"/\" \"INBOX\"\r\n* LIST (\\HasNoChildren) \"/\" \"Projects\"\r\n* LIST (\\HasNoChildren \\All) \"/\" \"[Gmail]/All Mail\"\r\n* LIST (\\HasNoChildren \\Junk) \"/\" \"[Gmail]/Spam\"\r\n* LIST (\\HasNoChildren \\Trash) \"/\" \"[Gmail]/Trash\"\r\n%s OK listed\r\n", tag)
			case strings.HasPrefix(upper, "SELECT "):
				values := quotedIMAPValues(command)
				selected = values[0]
				if selected == "[Gmail]/All Mail" {
					_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 10] next\r\n* OK [HIGHESTMODSEQ 10] current\r\n%s OK selected\r\n", tag)
				} else {
					_, _ = fmt.Fprintf(connection, "* 0 EXISTS\r\n* OK [UIDVALIDITY 88] valid\r\n* OK [UIDNEXT 1] next\r\n* OK [HIGHESTMODSEQ 1] current\r\n%s OK selected\r\n", tag)
				}
			case upper == "UID SEARCH UID 1:*" && selected == "[Gmail]/All Mail":
				_, _ = fmt.Fprintf(connection, "* SEARCH 9\r\n%s OK searched\r\n", tag)
			case upper == "UID SEARCH UID 1:*":
				_, _ = fmt.Fprintf(connection, "* SEARCH\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(upper, "UID FETCH 9 (") && strings.Contains(upper, "BODY.PEEK[]"):
				_, _ = fmt.Fprintf(connection, "* 1 FETCH (UID 9 FLAGS (\\Seen) X-GM-MSGID 123456 X-GM-LABELS (\\Inbox Projects) RFC822.SIZE %d INTERNALDATE \"21-Aug-2026 12:00:00 +0000\" BODY[] {%d}\r\n", len(raw), len(raw))
				_, _ = connection.Write(raw)
				_, _ = fmt.Fprintf(connection, ")\r\n%s OK fetched\r\n", tag)
			case strings.HasPrefix(upper, "UID FETCH 1:9 (") && strings.Contains(upper, "CHANGEDSINCE 0"):
				_, _ = fmt.Fprintf(connection, "* 1 FETCH (UID 9 FLAGS (\\Seen) MODSEQ (10) X-GM-MSGID 123456 X-GM-LABELS (\\Inbox Projects))\r\n%s OK changed\r\n", tag)
			case upper == "UID SEARCH UID 1:9":
				_, _ = fmt.Fprintf(connection, "* SEARCH 9\r\n%s OK searched\r\n", tag)
			case upper == "LOGOUT":
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{MaxMessages: 10, PageSize: 20})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, MailboxAddress: "owner@gmail.com", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	sink := &captureSink{}
	if err := connector.Sync(context.Background(), accountsync.SyncRequest{Account: account, Credentials: accounts.Credentials{AccessToken: "access-token"}}, sink); err != nil {
		t.Fatal(err)
	}
	var imported *accountsync.Message
	for _, batch := range sink.batches {
		if len(batch.Messages) > 0 {
			imported = &batch.Messages[0]
		}
	}
	if imported == nil || imported.StableID != "gmail:123456" || imported.RemoteStableID != "[Gmail]/All Mail:77:9" || fmt.Sprint(imported.FolderStableIDs) != "[INBOX Projects [Gmail]/All Mail]" {
		t.Fatalf("imported message = %#v", imported)
	}
	commandsMu.Lock()
	joined := strings.Join(commands, "\n")
	commandsMu.Unlock()
	if strings.Contains(joined, `SELECT "Projects" (CONDSTORE)\nUID SEARCH`) || strings.Contains(joined, `UID FETCH 9 (UID FLAGS RFC822.SIZE INTERNALDATE BODY.PEEK[])`) {
		t.Fatalf("Gmail commands did not use canonical label sync: %s", joined)
	}
}

func TestGmailMoveAddsDestinationLabelRemovesInboxAndKeepsCanonicalIdentity(t *testing.T) {
	commands := []string{}
	var commandsMu sync.Mutex
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.TrimSpace(strings.TrimSuffix(line, "\r\n"))[len(fields[0])+1:]
			upper := strings.ToUpper(command)
			commandsMu.Lock()
			commands = append(commands, command)
			commandsMu.Unlock()
			switch {
			case strings.HasPrefix(upper, "AUTHENTICATE XOAUTH2 "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case upper == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 X-GM-EXT-1\r\n%s OK capable\r\n", tag)
			case strings.HasPrefix(upper, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\All) \"/\" \"[Gmail]/All Mail\"\r\n* LIST (\\HasNoChildren) \"/\" \"Projects\"\r\n%s OK listed\r\n", tag)
			case strings.HasPrefix(upper, "SELECT "):
				_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n%s OK selected\r\n", tag)
			case upper == "UID SEARCH X-GM-MSGID 123456":
				_, _ = fmt.Fprintf(connection, "* SEARCH 9\r\n%s OK searched\r\n", tag)
			case upper == `UID STORE 9 +X-GM-LABELS.SILENT ("PROJECTS")`:
				_, _ = fmt.Fprintf(connection, "%s OK label added\r\n", tag)
			case upper == `UID STORE 9 -X-GM-LABELS.SILENT ("\\INBOX" "\\TRASH" "\\SPAM")`:
				_, _ = fmt.Fprintf(connection, "%s OK inbox removed\r\n", tag)
			case upper == "LOGOUT":
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
	account := accounts.ExternalAccount{Provider: accounts.ProviderGmail, MailboxAddress: "owner@gmail.com", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	result, err := connector.ApplyMutation(context.Background(), accountsync.MutationContext{Account: account, Credentials: accounts.Credentials{AccessToken: "access-token"},
		Mutation: accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationMove, MessageStableIDs: []string{"gmail:123456"}, TargetFolderID: "Projects"}}})
	if err != nil || result.Outcome != accountsync.OutcomeApplied || fmt.Sprint(result.ResultStableIDs) != "[gmail:123456]" {
		t.Fatalf("ApplyMutation() = %#v, %v", result, err)
	}
	commandsMu.Lock()
	joined := strings.ToUpper(strings.Join(commands, "\n"))
	commandsMu.Unlock()
	if !strings.Contains(joined, `+X-GM-LABELS.SILENT ("PROJECTS")`) || !strings.Contains(joined, `-X-GM-LABELS.SILENT ("\\INBOX" "\\TRASH" "\\SPAM")`) || strings.Contains(joined, "UID MOVE") {
		t.Fatalf("commands = %s", joined)
	}
}

func TestGmailReconcileMoveRecognizesCommittedLabelChange(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.TrimSpace(strings.TrimSuffix(line, "\r\n"))[len(fields[0])+1:]
			switch upper := strings.ToUpper(command); {
			case strings.HasPrefix(upper, "AUTHENTICATE XOAUTH2 "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case upper == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 X-GM-EXT-1\r\n%s OK capable\r\n", tag)
			case strings.HasPrefix(upper, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\All) \"/\" \"[Gmail]/All Mail\"\r\n* LIST (\\HasNoChildren) \"/\" \"Projects\"\r\n%s OK listed\r\n", tag)
			case strings.HasPrefix(upper, "SELECT "):
				_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n%s OK selected\r\n", tag)
			case upper == "UID SEARCH X-GM-MSGID 123456":
				_, _ = fmt.Fprintf(connection, "* SEARCH 9\r\n%s OK searched\r\n", tag)
			case upper == "UID FETCH 9 (X-GM-LABELS)":
				_, _ = fmt.Fprintf(connection, "* 1 FETCH (UID 9 X-GM-LABELS (Projects))\r\n%s OK fetched\r\n", tag)
			case upper == "LOGOUT":
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
	account := accounts.ExternalAccount{Provider: accounts.ProviderGmail, MailboxAddress: "owner@gmail.com", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	mutation := accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationMove, MessageStableIDs: []string{"gmail:123456"}, TargetFolderID: "Projects"}}
	result, err := connector.ReconcileMutation(context.Background(), accountsync.MutationContext{Account: account, Credentials: accounts.Credentials{AccessToken: "access-token"}, Mutation: mutation})
	if err != nil || result.Outcome != accountsync.OutcomeApplied || fmt.Sprint(result.ResultStableIDs) != "[gmail:123456]" {
		t.Fatalf("ReconcileMutation() = %#v, %v", result, err)
	}
}

func TestGmailFailsClosedWithoutStableIdentityExtension(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(line, "\r\n"))[len(fields[0])+1:])
			switch {
			case strings.HasPrefix(command, "AUTHENTICATE XOAUTH2 "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case command == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 IDLE\r\n%s OK capable\r\n", tag)
			case command == "LOGOUT":
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
	account := accounts.ExternalAccount{Provider: accounts.ProviderGmail, MailboxAddress: "owner@gmail.com", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	_, err := connector.ApplyMutation(context.Background(), accountsync.MutationContext{Account: account, Credentials: accounts.Credentials{AccessToken: "access-token"}, Mutation: accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationSetRead, MessageStableIDs: []string{"gmail:123456"}, Value: true}}})
	if err == nil || !strings.Contains(err.Error(), "gmail_extension_missing") {
		t.Fatalf("ApplyMutation() error = %v", err)
	}
}

func TestIMAPConnectorIDLEWakesDurableSchedulerOnMailboxChange(t *testing.T) {
	doneSeen := make(chan struct{}, 1)
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		idleTag := ""
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.TrimSpace(line) == "DONE" {
				doneSeen <- struct{}{}
				_, _ = fmt.Fprintf(connection, "%s OK idle complete\r\n", idleTag)
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
			switch {
			case strings.HasPrefix(command, "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case command == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 IDLE\r\n%s OK capable\r\n", tag)
			case strings.HasPrefix(command, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\Inbox) \"/\" \"INBOX\"\r\n%s OK listed\r\n", tag)
			case command == `SELECT "INBOX"`:
				_, _ = fmt.Fprintf(connection, "* 3 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 4] next\r\n%s OK selected\r\n", tag)
			case command == "IDLE":
				idleTag = tag
				_, _ = fmt.Fprint(connection, "+ idling\r\n* 4 EXISTS\r\n")
			case command == "LOGOUT":
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, err := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{IdleTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	account := accounts.ExternalAccount{Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	changed, err := connector.WaitForChange(context.Background(), account, accounts.Credentials{Password: "secret"})
	if err != nil || !changed {
		t.Fatalf("WaitForChange() = %v, %v", changed, err)
	}
	select {
	case <-doneSeen:
	case <-time.After(time.Second):
		t.Fatal("client did not terminate IDLE with DONE")
	}
}

func TestClassifyIdleLineCoversChangesCompletionKeepaliveAndClosure(t *testing.T) {
	tests := []struct {
		line    string
		want    idleLineResult
		wantErr bool
	}{
		{line: "* 4 EXISTS", want: idleChanged},
		{line: "* 2 FETCH (FLAGS (\\Seen))", want: idleChanged},
		{line: "c0004 OK idle complete", want: idleCompleted},
		{line: "* OK still here", want: idleContinues},
		{line: "* BYE maintenance", want: idleCompleted, wantErr: true},
	}
	for _, test := range tests {
		got, err := classifyIdleLine("c0004", test.line)
		if got != test.want || (err != nil) != test.wantErr {
			t.Errorf("classifyIdleLine(%q) = %v, %v", test.line, got, err)
		}
	}
}

func TestIMAPIdleTimeoutEndsWithDONEWithoutReportingAChange(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(server)
		line, err := reader.ReadString('\n')
		if err != nil || !strings.Contains(line, " IDLE") {
			done <- fmt.Errorf("idle command = %q, %v", line, err)
			return
		}
		_, _ = fmt.Fprint(server, "+ idling\r\n")
		line, err = reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != "DONE" {
			done <- fmt.Errorf("idle completion = %q, %v", line, err)
			return
		}
		_, _ = fmt.Fprint(server, "c0001 OK idle complete\r\n")
		done <- nil
	}()
	session := &imapSession{connection: client, reader: protocolReader(client)}
	changed, err := session.waitForChange(context.Background(), 20*time.Millisecond)
	if err != nil || changed {
		t.Fatalf("waitForChange() = %v, %v", changed, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestIMAPConnectorCheckpointsBoundedCONDSTORESweep(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
			switch {
			case strings.HasPrefix(command, "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case command == "CAPABILITY":
				_, _ = fmt.Fprintf(connection, "* CAPABILITY IMAP4REV1 CONDSTORE\r\n%s OK capable\r\n", tag)
			case strings.HasPrefix(command, "LIST "):
				_, _ = fmt.Fprintf(connection, "* LIST (\\HasNoChildren \\Inbox) \"/\" \"INBOX\"\r\n%s OK listed\r\n", tag)
			case command == `SELECT "INBOX" (CONDSTORE)`:
				_, _ = fmt.Fprintf(connection, "* 2 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n* OK [UIDNEXT 4] next\r\n* OK [HIGHESTMODSEQ 12] current\r\n%s OK selected\r\n", tag)
			case strings.HasPrefix(command, "UID SEARCH UID 4:"):
				_, _ = fmt.Fprintf(connection, "* SEARCH\r\n%s OK searched\r\n", tag)
			case command == "UID FETCH 1:3 (UID FLAGS MODSEQ) (CHANGEDSINCE 10)":
				_, _ = fmt.Fprintf(connection, "* 1 FETCH (UID 1 FLAGS (\\Seen) MODSEQ (11))\r\n%s OK changed\r\n", tag)
			case command == "UID SEARCH UID 1:3":
				_, _ = fmt.Fprintf(connection, "* SEARCH 1 3\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(command, "LOGOUT"):
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{MaxMessages: 10, PageSize: 3})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net", Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	checkpoint := accountsync.Checkpoint{Cursor: `{"folders":{"INBOX":{"uidValidity":77,"lastUid":3,"reconcileUid":1,"highestModSeq":10}}}`, Version: 4}
	sink := &captureSink{cursor: checkpoint.Cursor}
	if err := connector.Sync(context.Background(), accountsync.SyncRequest{Account: account, Credentials: accounts.Credentials{Password: "secret"}, Checkpoint: checkpoint}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 2 || len(sink.batches[1].Updates) != 1 || len(sink.batches[1].Windows) != 1 || len(sink.batches[1].Windows[0].PresentIDs) != 2 || !strings.Contains(sink.cursor, `"highestModSeq":12`) || strings.Contains(sink.cursor, "modSeqTarget") {
		t.Fatalf("batches=%#v cursor=%s", sink.batches, sink.cursor)
	}
}

func TestAdvanceReconcileCursorDefersModSeqUntilSweepCompletes(t *testing.T) {
	state := imapFolderCursor{UIDValidity: 77, LastUID: 5, ReconcileUID: 1, HighestModSeq: 10, ModSeqTarget: 12, ModSeqUID: 1}
	partial := advanceReconcileCursor(imapReconcileWindow{state: state, maximumUID: 5, start: 1, end: 2, modSeqScan: true})
	if partial.HighestModSeq != 10 || partial.ModSeqTarget != 12 || partial.ModSeqUID != 3 {
		t.Fatalf("partial cursor = %#v", partial)
	}
	completed := advanceReconcileCursor(imapReconcileWindow{state: partial, maximumUID: 5, start: 3, end: 5, modSeqScan: true})
	if completed.HighestModSeq != 12 || completed.ModSeqTarget != 0 || completed.ModSeqUID != 0 {
		t.Fatalf("completed cursor = %#v", completed)
	}
}

func TestAdvanceEmptyMailboxCommitsUIDNextCheckpoint(t *testing.T) {
	run := imapSyncRun{cursor: imapCursor{Folders: map[string]imapFolderCursor{"INBOX": {UIDValidity: 77, LastUID: 3}}}, currentCursor: "before", completed: 4}
	sink := &captureSink{cursor: run.currentCursor}
	err := advanceEmptyMailbox(context.Background(), accountsync.Folder{StableID: "INBOX"}, selectedFolder{uidNext: 8}, run.cursor.Folders["INBOX"], nil, &run, sink)
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 1 || !strings.Contains(run.currentCursor, `"lastUid":7`) || sink.batches[0].Completed != 4 {
		t.Fatalf("batches=%#v cursor=%s", sink.batches, run.currentCursor)
	}
}

func TestParseUIDSetIsBoundedAndCanonical(t *testing.T) {
	uids, err := parseUIDSet("7,2:4,3", 10)
	if err != nil || fmt.Sprint(uids) != "[2 3 4 7]" {
		t.Fatalf("uids=%v err=%v", uids, err)
	}
	for _, value := range []string{"0", "9:2", "1:*", "1:1000"} {
		if _, err := parseUIDSet(value, 10); err == nil {
			t.Fatalf("parseUIDSet(%q) succeeded", value)
		}
	}
}

func TestParseGmailLabelsPreservesQuotedNamesAndSystemLabels(t *testing.T) {
	labels := parseGmailLabels(`* 1 FETCH (X-GM-LABELS (\\Inbox "Team (East)" "R&D"))`)
	if fmt.Sprint(labels) != `[\\Inbox Team (East) R&D]` {
		t.Fatalf("labels = %#v", labels)
	}
}

func TestGmailMoveLabelRulesCoverSystemAndCustomDestinations(t *testing.T) {
	tests := []struct {
		folder accountsync.Folder
		labels []string
		want   bool
	}{
		{folder: accountsync.Folder{Role: "inbox"}, labels: []string{"\\Inbox", "Projects"}, want: true},
		{folder: accountsync.Folder{Role: "trash"}, labels: []string{"\\Trash"}, want: true},
		{folder: accountsync.Folder{Role: "all"}, labels: []string{"Projects"}, want: true},
		{folder: accountsync.Folder{StableID: "Projects"}, labels: []string{"Projects"}, want: true},
		{folder: accountsync.Folder{StableID: "Projects"}, labels: []string{"Projects", "\\Inbox"}},
		{folder: accountsync.Folder{Role: "spam"}, labels: []string{"\\Spam", "\\Trash"}},
	}
	for _, test := range tests {
		if got := gmailMoveApplied(test.labels, test.folder); got != test.want {
			t.Errorf("gmailMoveApplied(%v, %#v) = %t, want %t", test.labels, test.folder, got, test.want)
		}
	}
}

func TestDecodeIMAPCursorRejectsInconsistentModSeqState(t *testing.T) {
	if _, err := decodeIMAPCursor(`{"folders":{"INBOX":{"uidValidity":0}}}`); err != nil {
		t.Fatalf("empty folder epoch should be valid: %v", err)
	}
	for _, value := range []string{
		`{"folders":{"INBOX":{"uidValidity":0,"lastUid":1}}}`,
		`{"folders":{"INBOX":{"uidValidity":77,"highestModSeq":10,"modSeqTarget":12}}}`,
		`{"folders":{"INBOX":{"uidValidity":77,"highestModSeq":10,"modSeqTarget":9,"modSeqUid":1}}}`,
	} {
		if _, err := decodeIMAPCursor(value); err == nil {
			t.Fatalf("decodeIMAPCursor(%s) succeeded", value)
		}
	}
}

func TestIMAPConnectorDeleteUsesExactUIDExpungeAndRollsBackWithoutUIDPLUS(t *testing.T) {
	tests := []struct {
		name            string
		expungeAccepted bool
		uidValidity     uint64
		wantOutcome     accountsync.MutationOutcome
		wantCode        string
		wantDelete      bool
		wantRollback    bool
	}{
		{name: "UIDPLUS expunge", expungeAccepted: true, uidValidity: 77, wantOutcome: accountsync.OutcomeApplied, wantDelete: true},
		{name: "unsupported UIDPLUS", uidValidity: 77, wantOutcome: accountsync.OutcomeNotApplied, wantCode: "imap_uid_expunge_unsupported", wantDelete: true, wantRollback: true},
		{name: "stale mailbox epoch", uidValidity: 88, wantOutcome: accountsync.OutcomeNotApplied, wantCode: "imap_uidvalidity_changed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var commandMu sync.Mutex
			commands := []string{}
			address, stop := serveProtocol(t, func(connection net.Conn) {
				reader := bufio.NewReader(connection)
				_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					fields := strings.Fields(line)
					if len(fields) < 2 {
						return
					}
					tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
					commandMu.Lock()
					commands = append(commands, command)
					commandMu.Unlock()
					switch {
					case strings.HasPrefix(command, "LOGIN "):
						_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
					case strings.HasPrefix(command, "SELECT "):
						_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY %d] valid\r\n%s OK selected\r\n", test.uidValidity, tag)
					case command == "UID STORE 9 +FLAGS.SILENT (\\DELETED)":
						_, _ = fmt.Fprintf(connection, "%s OK marked\r\n", tag)
					case command == "UID EXPUNGE 9" && test.expungeAccepted:
						_, _ = fmt.Fprintf(connection, "* 1 EXPUNGE\r\n%s OK expunged\r\n", tag)
					case command == "UID EXPUNGE 9":
						_, _ = fmt.Fprintf(connection, "%s BAD UIDPLUS unavailable\r\n", tag)
					case command == "UID STORE 9 -FLAGS.SILENT (\\DELETED)":
						_, _ = fmt.Fprintf(connection, "%s OK restored\r\n", tag)
					case strings.HasPrefix(command, "LOGOUT"):
						_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
						return
					default:
						_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
					}
				}
			})
			defer stop()
			host, port, _ := net.SplitHostPort(address)
			connector, err := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
			if err != nil {
				t.Fatal(err)
			}
			account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net",
				Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
			result, err := connector.ApplyMutation(context.Background(), accountsync.MutationContext{
				Account: account, Credentials: accounts.Credentials{Password: "secret"},
				Mutation: accountsync.Mutation{MutationRequest: accountsync.MutationRequest{
					Kind: accountsync.MutationDelete, MessageStableIDs: []string{"INBOX:77:9"},
				}},
			})
			if err != nil || result.Outcome != test.wantOutcome || result.Code != test.wantCode {
				t.Fatalf("ApplyMutation() = %#v, %v", result, err)
			}
			commandMu.Lock()
			joined := strings.Join(commands, "\n")
			commandMu.Unlock()
			deleteAttempted := strings.Contains(joined, "UID STORE 9 +FLAGS.SILENT (\\DELETED)\nUID EXPUNGE 9")
			if deleteAttempted != test.wantDelete {
				t.Fatalf("commands = %q", joined)
			}
			rolledBack := strings.Contains(joined, "UID STORE 9 -FLAGS.SILENT (\\DELETED)")
			if rolledBack != test.wantRollback {
				t.Fatalf("rollback present = %v, commands = %q", rolledBack, joined)
			}
		})
	}
}

func TestIMAPConnectorCapturesAndRequiresMoveCOPYUID(t *testing.T) {
	for _, test := range []struct {
		name       string
		moveReply  string
		want       accountsync.MutationOutcome
		wantCode   string
		wantStable string
	}{
		{name: "captures destination identity", moveReply: "OK [COPYUID 88 9 41] moved", want: accountsync.OutcomeApplied, wantStable: "Archive:88:41"},
		{name: "fails closed without destination identity", moveReply: "OK moved", want: accountsync.OutcomeUnknown, wantCode: "imap_copyuid_missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			address, stop := serveProtocol(t, func(connection net.Conn) {
				reader := bufio.NewReader(connection)
				_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					fields := strings.Fields(line)
					if len(fields) < 2 {
						return
					}
					tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
					switch {
					case strings.HasPrefix(command, "LOGIN "):
						_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
					case strings.HasPrefix(command, "SELECT "):
						_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n%s OK selected\r\n", tag)
					case command == `UID MOVE 9 "ARCHIVE"`:
						_, _ = fmt.Fprintf(connection, "%s %s\r\n", tag, test.moveReply)
					case strings.HasPrefix(command, "LOGOUT"):
						_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
						return
					default:
						_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
					}
				}
			})
			defer stop()
			host, port, _ := net.SplitHostPort(address)
			connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
			account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net",
				Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
			result, err := connector.ApplyMutation(context.Background(), accountsync.MutationContext{Account: account, Credentials: accounts.Credentials{Password: "secret"},
				Mutation: accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationMove, MessageStableIDs: []string{"INBOX:77:9"}, TargetFolderID: "Archive"}}})
			if err != nil || result.Outcome != test.want || result.Code != test.wantCode {
				t.Fatalf("ApplyMutation() = %#v, %v", result, err)
			}
			if test.wantStable != "" && (len(result.ResultStableIDs) != 1 || result.ResultStableIDs[0] != test.wantStable) {
				t.Fatalf("result identities = %#v", result.ResultStableIDs)
			}
		})
	}
}

func TestIMAPConnectorReconcilesMoveUsingPersistedDestinationIdentity(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		selected := ""
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.ToUpper(strings.Join(fields[1:], " "))
			switch {
			case strings.HasPrefix(command, "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case command == `SELECT "INBOX"`:
				selected = "INBOX"
				_, _ = fmt.Fprintf(connection, "* 0 EXISTS\r\n* OK [UIDVALIDITY 77] valid\r\n%s OK selected\r\n", tag)
			case command == `SELECT "ARCHIVE"`:
				selected = "ARCHIVE"
				_, _ = fmt.Fprintf(connection, "* 1 EXISTS\r\n* OK [UIDVALIDITY 88] valid\r\n%s OK selected\r\n", tag)
			case strings.HasPrefix(command, "UID SEARCH UID ") && selected == "INBOX":
				_, _ = fmt.Fprintf(connection, "* SEARCH\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(command, "UID SEARCH UID ") && selected == "ARCHIVE":
				_, _ = fmt.Fprintf(connection, "* SEARCH 41\r\n%s OK searched\r\n", tag)
			case strings.HasPrefix(command, "LOGOUT"):
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, _ := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net",
		Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}}
	result, err := connector.ReconcileMutation(context.Background(), accountsync.MutationContext{Account: account, Credentials: accounts.Credentials{Password: "secret"},
		Mutation: accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationMove, MessageStableIDs: []string{"INBOX:77:9"}, TargetFolderID: "Archive"}, ResultStableIDs: []string{"Archive:88:41"}}})
	if err != nil || result.Outcome != accountsync.OutcomeApplied || len(result.ResultStableIDs) != 1 || result.ResultStableIDs[0] != "Archive:88:41" {
		t.Fatalf("ReconcileMutation() = %#v, %v", result, err)
	}
}

func TestIMAPConnectorAppliesAndReconcilesDurableFolderCreation(t *testing.T) {
	var stateMu sync.Mutex
	created := false
	address, stop := serveRepeatedProtocol(t, 2, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK fixture ready\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
			switch {
			case strings.HasPrefix(strings.ToUpper(command), "LOGIN "):
				_, _ = fmt.Fprintf(connection, "%s OK authenticated\r\n", tag)
			case strings.HasPrefix(strings.ToUpper(command), "LIST "):
				_, _ = fmt.Fprint(connection, "* LIST (\\HasChildren) \"/\" \"Projects\"\r\n")
				stateMu.Lock()
				if created {
					_, _ = fmt.Fprint(connection, "* LIST (\\HasNoChildren) \"/\" \"Projects/&ZeVnLIqe-\"\r\n")
				}
				stateMu.Unlock()
				_, _ = fmt.Fprintf(connection, "%s OK listed\r\n", tag)
			case command == `CREATE "Projects/&ZeVnLIqe-"`:
				stateMu.Lock()
				created = true
				stateMu.Unlock()
				_, _ = fmt.Fprintf(connection, "%s OK created\r\n", tag)
			case strings.HasPrefix(strings.ToUpper(command), "LOGOUT"):
				_, _ = fmt.Fprintf(connection, "* BYE closing\r\n%s OK logout\r\n", tag)
				return
			default:
				_, _ = fmt.Fprintf(connection, "%s BAD unexpected\r\n", tag)
			}
		}
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	connector, err := NewIMAPConnector(localPasswordValidator(t, 2*time.Second), IMAPSyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	request := accountsync.MutationContext{
		Account: accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderIMAP, MailboxAddress: "owner@example.net",
			Connection: accounts.ConnectionSettings{IMAP: &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}}},
		Credentials: accounts.Credentials{Password: "secret"},
		Mutation: accountsync.Mutation{MutationRequest: accountsync.MutationRequest{Kind: accountsync.MutationFolderCreate,
			ParentStableID: "Projects", FolderName: "日本語", NewFolderPath: "Projects/&ZeVnLIqe-"}},
	}
	result, err := connector.ApplyMutation(context.Background(), request)
	if err != nil || result.Outcome != accountsync.OutcomeApplied {
		t.Fatalf("ApplyMutation() = %#v, %v", result, err)
	}
	result, err = connector.ReconcileMutation(context.Background(), request)
	if err != nil || result.Outcome != accountsync.OutcomeApplied {
		t.Fatalf("ReconcileMutation() = %#v, %v", result, err)
	}
	folder, ok := parseListFolder(`* LIST (\HasNoChildren) "/" "Projects/&ZeVnLIqe-"`)
	if !ok || folder.Name != "Projects/日本語" || folder.StableID != "Projects/&ZeVnLIqe-" {
		t.Fatalf("parseListFolder() = %#v, %v", folder, ok)
	}
	for _, fixture := range []struct {
		line      string
		stableID  string
		delimiter string
	}{
		{line: `* LIST (\\HasNoChildren \\Inbox) "/" INBOX`, stableID: "INBOX", delimiter: "/"},
		{line: `* LIST (\\Noselect) NIL "Shared root"`, stableID: "Shared root", delimiter: ""},
	} {
		folder, ok := parseListFolder(fixture.line)
		if !ok || folder.StableID != fixture.stableID || folder.Delimiter != fixture.delimiter {
			t.Errorf("parseListFolder(%q) = %#v, %v", fixture.line, folder, ok)
		}
	}
}

func serveRepeatedProtocol(t *testing.T, connections int, handler func(net.Conn)) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for count := 0; count < connections; count++ {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			handler(connection)
			_ = connection.Close()
		}
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}

type captureSink struct {
	batches []accountsync.Batch
	cursor  string
}

func (sink *captureSink) Commit(_ context.Context, batch accountsync.Batch) error {
	if batch.CursorBefore != sink.cursor {
		return accountsync.ErrCheckpointConflict
	}
	sink.batches = append(sink.batches, batch)
	sink.cursor = batch.CursorAfter
	return nil
}
