import (
	"regexp"
	"sort"
	"strings"
)

// ThreadsRecords groups authorized messages into conversations by their RFC
// Message-ID relationships. Each edge key carries the hosted domain or the
// connected account, so two tenants never share a conversation.
type ThreadsRecords struct{}

// GroupMessagesIntoThreads answers the conversations of emails, newest
// first, each with its messages oldest first.
func (records ThreadsRecords) GroupMessagesIntoThreads(emails []map[string]any, ownerEmail string) []Thread {
	text := TextValue{}
	level := PersonalLevel{}
	threads := []Thread{}
	for _, threadEmails := range records.ConnectedComponents(emails) {
		sort.SliceStable(threadEmails, func(left, right int) bool {
			return text.Of(threadEmails[left]["receivedAt"]) < text.Of(threadEmails[right]["receivedAt"])
		})
		first := threadEmails[0]
		id := "legacy:" + records.MessageScope(first) + ":" + strings.ToLower(records.BaseSubject(text.Of(first["subject"])))
		for _, email := range threadEmails {
			if text.Truthy(email["threadId"]) {
				id = text.Of(email["threadId"])
				break
			}
		}
		participants := []string{}
		for _, email := range threadEmails {
			participants = append(participants, level.MessageParticipants(email)...)
		}
		mailbox := ownerEmail
		if text.Truthy(first["providerAccountId"]) {
			mailbox = text.Of(first["mailboxAddress"])
		}
		threads = append(threads, Thread{
			Id: id, Subject: records.BaseSubject(text.Of(first["subject"])), Count: len(threadEmails),
			LatestEmail: threadEmails[len(threadEmails)-1], Emails: threadEmails,
			Participants: MailAddresses{}.Unique(participants), PersonalLevel: level.PersonalLevelForThread(threadEmails, mailbox),
		})
	}
	sort.SliceStable(threads, func(left, right int) bool {
		return text.Of(threads[left].LatestEmail["receivedAt"]) > text.Of(threads[right].LatestEmail["receivedAt"])
	})
	return threads
}

// ConnectedComponents answers the messages grouped by their Message-ID,
// stored thread id and reference edges (subject, for historical messages
// without any RFC metadata), in the order each group first appears.
func (records ThreadsRecords) ConnectedComponents(emails []map[string]any) [][]map[string]any {
	text := TextValue{}
	groups := MessageGroups{}.Of(len(emails))
	byMessageId := map[string]int{}
	byThreadId := map[string]int{}
	legacyBySubject := map[string]int{}
	for index, email := range emails {
		scope := records.MessageScope(email) + "\x00"
		if text.Truthy(email["rfcMessageId"]) {
			groups.Connect(byMessageId, scope+text.Of(email["rfcMessageId"]), index)
		}
		if text.Truthy(email["threadId"]) {
			groups.Connect(byThreadId, scope+text.Of(email["threadId"]), index)
		}
		if !records.hasRfcMetadata(email) {
			groups.Connect(legacyBySubject, scope+strings.ToLower(records.BaseSubject(text.Of(email["subject"]))), index)
		}
	}
	for index, email := range emails {
		scope := records.MessageScope(email) + "\x00"
		for _, referenced := range append([]any{email["inReplyTo"]}, records.references(email["references"])...) {
			if !text.Truthy(referenced) {
				continue
			}
			if linked, found := byMessageId[scope+text.Of(referenced)]; found {
				groups.Union(index, linked)
			}
		}
	}
	return groups.Grouped(emails)
}

func (records ThreadsRecords) hasRfcMetadata(email map[string]any) bool {
	text := TextValue{}
	return text.Truthy(email["rfcMessageId"]) || text.Truthy(email["inReplyTo"]) || len(records.references(email["references"])) > 0 || text.Truthy(email["threadId"])
}

// references answers the items `...(references || [])` spreads: a list's
// items, or a text's letters.
func (ThreadsRecords) references(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case string:
		letters := []any{}
		for _, letter := range typed {
			letters = append(letters, string(letter))
		}
		return letters
	}
	return nil
}

// MessageScope answers the tenant a message belongs to: its connected
// account, or its hosted domain.
func (ThreadsRecords) MessageScope(email map[string]any) string {
	text := TextValue{}
	if account := text.Trim(text.Of(text.Or(email["providerAccountId"], ""))); account != "" {
		return "external:" + account
	}
	return "hosted:" + strings.ToLower(text.Of(text.Or(email["domain"], "")))
}

// BaseSubject answers a subject without its leading Re:, Fw: and Fwd:
// prefixes (the whole subject when nothing else is left).
func (ThreadsRecords) BaseSubject(subject string) string {
	text := TextValue{}
	stripped := text.Trim(regexp.MustCompile(`(?i)^(?:(?:re|fw|fwd):[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]*)+`).ReplaceAllString(subject, ""))
	if stripped == "" {
		return text.Trim(subject)
	}
	return stripped
}

// InFolder answers the threads with a message in the folder (each domain's
// own folder first); no folder or "all" keeps every thread.
func (ThreadsRecords) InFolder(threads []Thread, folder string, folderByDomain map[string]string) []Thread {
	if folder == "" || folder == "all" {
		return threads
	}
	text := TextValue{}
	kept := []Thread{}
	for _, thread := range threads {
		for _, email := range thread.Emails {
			selected := folderByDomain[text.Of(email["domain"])]
			if selected == "" {
				selected = folder
			}
			if text.Of(text.Or(email["folder"], "inbox")) == selected {
				kept = append(kept, thread)
				break
			}
		}
	}
	return kept
}

// Holding answers the first thread with a message of the logical id, or nil.
func (ThreadsRecords) Holding(threads []Thread, emailId string) *Thread {
	for index := range threads {
		for _, email := range threads[index].Emails {
			if email["id"] == emailId {
				return &threads[index]
			}
		}
	}
	return nil
}
