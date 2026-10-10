import (
	"errors"
	"sort"
	"strings"
)

// McpMailbox runs the MCP tools for one authenticated capability. Every
// tool re-applies exact-owner, domain, folder-exclusion checks on the data
// it reads; the endpoint already checked the access level.
type McpMailbox struct {
	Principal   McpPrincipal
	Users       *UsersHelper
	Emails      *EmailsHelper
	Threads     *ThreadsHelper
	Attachments *AttachmentsHelper
	Scheduled   *ScheduledHelper
}

// Run answers the result of one tool with already parsed arguments.
func (mailbox McpMailbox) Run(name string, input map[string]any) (any, error) {
	switch name {
	case "list_accounts":
		return mailbox.accounts()
	case "list_folders":
		return mailbox.folders(input)
	case "search_mail":
		return mailbox.search(input)
	case "list_threads":
		return mailbox.listThreads(input)
	case "get_thread":
		return mailbox.thread(RecordValues{}.Text(input["threadId"]))
	case "get_message":
		return mailbox.message(RecordValues{}.Text(input["messageId"]))
	case "get_message_tracking":
		return mailbox.tracking(RecordValues{}.Text(input["messageId"]))
	case "list_tracked_messages":
		return mailbox.trackedMessages(input)
	case "get_top_links":
		return mailbox.topLinks(input)
	case "list_scheduled_sends":
		return mailbox.scheduledSends()
	case "cancel_scheduled_send":
		return mailbox.cancelScheduled(RecordValues{}.Text(input["id"]))
	case "archive_threads", "trash_threads":
		return mailbox.moveToRole(mailbox.texts(input["threadIds"]), strings.TrimSuffix(name, "_threads"))
	case "move_to_folder":
		return mailbox.moveToFolder(input)
	case "set_labels":
		return mailbox.setLabels(mailbox.texts(input["threadIds"]), mailbox.texts(input["labels"]))
	case "set_unread":
		return mailbox.patchSelected(input, map[string]any{"read": input["unread"] != true})
	case "set_starred":
		return mailbox.patchSelected(input, map[string]any{"starred": input["starred"] == true})
	case "reply_to_thread":
		return mailbox.reply(input)
	case "schedule_send":
		if at := (EpochTime{}).Parse(RecordValues{}.Text(input["sendAt"])); at != at || at <= float64(EpochTime{}.Now()) {
			return nil, errors.New("sendAt must be in the future")
		}
	}
	return nil, errors.New("Drafts and sending over MCP are not available on this server yet")
}

func (mailbox McpMailbox) accounts() (any, error) {
	_, user, err := mailbox.Users.FindByEmail(mailbox.Principal.OwnerEmail)
	if err != nil {
		return nil, err
	}
	names, _ := user["mailboxNames"].(map[string]any)
	accounts := []any{}
	for _, domain := range mailbox.Principal.AllowedDomains {
		accounts = append(accounts, map[string]any{
			"id":         domain,
			"email":      mailbox.Principal.OwnerEmail,
			"name":       RecordValues{}.Or(names[domain], domain),
			"provider":   "caduceus",
			"usesLabels": true,
		})
	}
	return accounts, nil
}

func (mailbox McpMailbox) folders(input map[string]any) (any, error) {
	domains, err := mailbox.scopedDomains(input["accountId"])
	if err != nil {
		return nil, err
	}
	accounts, err := mailbox.categories().Accounts(mailbox.Principal.OwnerEmail, domains)
	if err != nil {
		return nil, err
	}
	result := []any{}
	for _, account := range accounts {
		domain := RecordValues{}.Text(account["domain"])
		folders, labels := []any{}, []any{}
		for _, category := range account["categories"].([]map[string]any) {
			path := RecordValues{}.Text(category["path"])
			if category["kind"] == "label" {
				labels = append(labels, map[string]any{"id": path, "name": path})
			} else if !mailbox.folderExcluded(domain, path) {
				folders = append(folders, map[string]any{"id": path, "name": path, "role": RecordValues{}.Or(category["role"], nil)})
			}
		}
		result = append(result, map[string]any{"accountId": domain, "folders": folders, "labels": labels})
	}
	return result, nil
}

func (mailbox McpMailbox) search(input map[string]any) (any, error) {
	domains, err := mailbox.scopedDomains(input["accountId"])
	if err != nil {
		return nil, err
	}
	folder, err := mailbox.requestedFolder(input["folder"])
	if err != nil {
		return nil, err
	}
	limit := mailbox.number(input["limit"])
	fetched := min(limit*2, 200)
	emails, err := mailbox.Emails.List(domains, EmailListFilters{Folder: folder, Query: RecordValues{}.Text(input["query"]), OwnerEmail: mailbox.Principal.OwnerEmail, Limit: &fetched})
	if err != nil {
		return nil, err
	}
	messages, err := mailbox.presentMessages(emails)
	return mailbox.first(messages, limit), err
}

func (mailbox McpMailbox) listThreads(input map[string]any) (any, error) {
	domains, err := mailbox.scopedDomains(input["accountId"])
	if err != nil {
		return nil, err
	}
	threads, err := mailbox.allowedThreads(domains, input["folder"])
	if err != nil {
		return nil, err
	}
	kept := []any{}
	for _, thread := range threads {
		if (input["unread"] == nil || mailbox.anyEmail(thread, "read", false) == input["unread"]) && (input["starred"] == nil || mailbox.anyEmail(thread, "starred", true) == input["starred"]) {
			kept = append(kept, mailbox.threadSummary(thread))
		}
	}
	offset := min(mailbox.number(input["offset"]), len(kept))
	return kept[offset:min(len(kept), offset+mailbox.number(input["limit"]))], nil
}

func (mailbox McpMailbox) thread(threadId string) (any, error) {
	thread, err := mailbox.findThread(threadId)
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, errors.New("Thread not found")
	}
	summary := mailbox.threadSummary(*thread)
	messages, err := mailbox.presentMessages(thread.Emails)
	summary["messages"] = messages
	return summary, err
}

func (mailbox McpMailbox) message(messageId string) (any, error) {
	email, err := mailbox.allowedMessage(messageId)
	if err != nil || email == nil {
		return nil, mailbox.notFound(err, "Message not found")
	}
	messages, err := mailbox.presentMessages([]map[string]any{email})
	if len(messages) == 0 {
		return nil, err
	}
	return messages[0], err
}

func (mailbox McpMailbox) tracking(messageId string) (any, error) {
	email, err := mailbox.allowedMessage(messageId)
	if err != nil || email == nil {
		return nil, mailbox.notFound(err, "Message not found")
	}
	return mailbox.trackingView(email), nil
}

func (mailbox McpMailbox) trackedMessages(input map[string]any) (any, error) {
	emails, err := mailbox.ownedEmails(input["accountId"])
	if err != nil {
		return nil, err
	}
	tracked := []any{}
	for _, email := range emails {
		if !mailbox.excluded(email) && (RecordValues{}.Truthy(email["openTracking"]) || RecordValues{}.Truthy(email["linkTracking"])) {
			view := mailbox.trackingView(email)
			for _, field := range []string{"id", "subject", "receivedAt"} {
				if value, present := email[field]; present {
					view[field] = value
				}
			}
			tracked = append(tracked, view)
		}
	}
	return mailbox.first(tracked, mailbox.number(input["limit"])), nil
}

func (mailbox McpMailbox) topLinks(input map[string]any) (any, error) {
	emails, err := mailbox.ownedEmails(input["accountId"])
	if err != nil {
		return nil, err
	}
	links := []map[string]any{}
	position := map[string]int{}
	for _, email := range emails {
		if mailbox.excluded(email) {
			continue
		}
		clicks, _ := RecordValues{}.List(email["clicks"])
		for _, item := range clicks {
			click, _ := item.(map[string]any)
			url := TextValue{}.Of(click["url"])
			if url == "" {
				continue
			}
			if _, seen := position[url]; !seen {
				position[url] = len(links)
				links = append(links, map[string]any{"url": url, "title": TextValue{}.Of(click["title"]), "clicks": 0})
			}
			links[position[url]]["clicks"] = links[position[url]]["clicks"].(int) + 1
		}
	}
	sort.SliceStable(links, func(left, right int) bool { return links[left]["clicks"].(int) > links[right]["clicks"].(int) })
	result := []any{}
	for _, link := range links {
		result = append(result, link)
	}
	return mailbox.first(result, mailbox.number(input["limit"])), nil
}

// scheduledSends lists nothing but the empty case: presenting an entry
// needs the outbox and follow-up services the JavaScript tool reached for
// but never had, so an owned entry fails as it did there.
func (mailbox McpMailbox) scheduledSends() (any, error) {
	records, err := mailbox.Scheduled.ListForOwner(mailbox.Principal.OwnerEmail)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		owned, err := mailbox.ownedScheduled(RecordValues{}.Text(record.Data["id"]))
		if err != nil {
			return nil, err
		}
		if owned != nil {
			return nil, errors.New("Scheduled sends cannot be listed over MCP on this server yet")
		}
	}
	return []any{}, nil
}

func (mailbox McpMailbox) cancelScheduled(id string) (any, error) {
	owned, err := mailbox.ownedScheduled(id)
	if err != nil {
		return nil, err
	}
	if owned == nil {
		return nil, errors.New("Scheduled send not found")
	}
	return nil, errors.New("Scheduled sends cannot be cancelled over MCP on this server yet")
}

func (mailbox McpMailbox) ownedScheduled(id string) (map[string]any, error) {
	documentId, entry, err := mailbox.Scheduled.FindById(id)
	if err != nil || documentId == "" || entry == nil || (RecordValues{}).Coalesce(entry["owner"], entry["sender"]) != mailbox.Principal.OwnerEmail {
		return nil, err
	}
	return entry, nil
}

func (mailbox McpMailbox) reply(input map[string]any) (any, error) {
	thread, err := mailbox.findThread(RecordValues{}.Text(input["threadId"]))
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, errors.New("Thread not found")
	}
	return nil, errors.New("Drafts and sending over MCP are not available on this server yet")
}

func (mailbox McpMailbox) moveToRole(threadIds []string, role string) (any, error) {
	threads, err := mailbox.resolveThreads(threadIds)
	if err != nil {
		return nil, err
	}
	return mailbox.patchThreads(threads, func(email map[string]any) (map[string]any, error) {
		folder, err := mailbox.categories().PathFor(mailbox.Principal.OwnerEmail, RecordValues{}.Text(email["domain"]), role)
		return map[string]any{"folder": folder}, err
	})
}

func (mailbox McpMailbox) moveToFolder(input map[string]any) (any, error) {
	domains, err := mailbox.scopedDomains(input["accountId"])
	if err != nil {
		return nil, err
	}
	destination := CategoryPath{}.Normalized(input["folder"])
	if destination == "" {
		return nil, errors.New("Destination folder must be a safe folder path")
	}
	if mailbox.folderExcluded(domains[0], destination) {
		return nil, errors.New("Destination folder is excluded")
	}
	threads, err := mailbox.resolveThreads(mailbox.texts(input["threadIds"]))
	if err != nil {
		return nil, err
	}
	for _, thread := range threads {
		for _, email := range thread.Emails {
			if email["domain"] != domains[0] {
				return nil, errors.New("Every selected thread must belong to the destination account")
			}
		}
	}
	return mailbox.patchThreads(threads, mailbox.fixed(map[string]any{"folder": destination}))
}

func (mailbox McpMailbox) setLabels(threadIds []string, labels []string) (any, error) {
	normalized := []any{}
	seen := map[string]bool{}
	unique := true
	for _, label := range labels {
		path := CategoryPath{}.Normalized(label)
		if path == "" {
			return nil, errors.New("Every label must be a safe label path")
		}
		unique = unique && !seen[path]
		seen[path] = true
		normalized = append(normalized, path)
	}
	if !unique {
		return nil, errors.New("labels must be unique")
	}
	threads, err := mailbox.resolveThreads(threadIds)
	if err != nil {
		return nil, err
	}
	domains := map[any]bool{}
	for _, thread := range threads {
		for _, email := range thread.Emails {
			domains[email["domain"]] = true
		}
	}
	if len(domains) != 1 {
		return nil, errors.New("Every selected thread must belong to one account")
	}
	return mailbox.patchThreads(threads, mailbox.fixed(map[string]any{"labels": normalized}))
}

func (mailbox McpMailbox) patchSelected(input map[string]any, patch map[string]any) (any, error) {
	threads, err := mailbox.resolveThreads(mailbox.texts(input["threadIds"]))
	if err != nil {
		return nil, err
	}
	return mailbox.patchThreads(threads, mailbox.fixed(patch))
}

func (McpMailbox) fixed(patch map[string]any) func(map[string]any) (map[string]any, error) {
	return func(map[string]any) (map[string]any, error) { return patch, nil }
}

func (mailbox McpMailbox) patchThreads(threads []Thread, patcher func(email map[string]any) (map[string]any, error)) (any, error) {
	updated := 0
	for _, thread := range threads {
		for _, email := range thread.Emails {
			documentId, current, err := mailbox.Emails.FindById(RecordValues{}.Text(email["id"]))
			if err != nil {
				return nil, err
			}
			if documentId == "" || current == nil {
				return nil, errors.New("Thread changed during the operation")
			}
			patch, err := patcher(current)
			if err != nil {
				return nil, err
			}
			if err := mailbox.Emails.Update(documentId, patch); err != nil {
				return nil, err
			}
			updated++
		}
	}
	return map[string]any{"updatedThreads": len(threads), "updatedMessages": updated}, nil
}

func (mailbox McpMailbox) resolveThreads(threadIds []string) ([]Thread, error) {
	seen := map[string]bool{}
	for _, id := range threadIds {
		if seen[id] {
			return nil, errors.New("threadIds must be unique")
		}
		seen[id] = true
	}
	all, err := mailbox.allowedThreads(mailbox.Principal.AllowedDomains, nil)
	if err != nil {
		return nil, err
	}
	threads := []Thread{}
	for _, id := range threadIds {
		found := false
		for _, thread := range all {
			if thread.Id == id {
				threads, found = append(threads, thread), true
				break
			}
		}
		if !found {
			return nil, errors.New("Every selected thread must be accessible")
		}
	}
	return threads, nil
}

func (mailbox McpMailbox) findThread(threadId string) (*Thread, error) {
	threads, err := mailbox.allowedThreads(mailbox.Principal.AllowedDomains, nil)
	if err != nil {
		return nil, err
	}
	for index := range threads {
		if threads[index].Id == threadId {
			return &threads[index], nil
		}
	}
	return nil, nil
}

func (mailbox McpMailbox) allowedThreads(domains []string, folder any) ([]Thread, error) {
	category, err := mailbox.requestedFolder(folder)
	if err != nil {
		return nil, err
	}
	threads, err := mailbox.Threads.List(domains, category, mailbox.Principal.OwnerEmail, nil)
	if err != nil {
		return nil, err
	}
	allowed := []Thread{}
	for _, thread := range threads {
		hidden := false
		for _, email := range thread.Emails {
			hidden = hidden || mailbox.excluded(email)
		}
		if !hidden {
			allowed = append(allowed, thread)
		}
	}
	return allowed, nil
}

func (mailbox McpMailbox) allowedMessage(id string) (map[string]any, error) {
	_, email, err := mailbox.Emails.FindById(id)
	if err != nil || email == nil {
		return nil, err
	}
	if !(TextValue{}).Includes(mailbox.Principal.AllowedDomains, email["domain"]) || mailbox.excluded(email) || !(EmailsRecords{}).CanAccessEmail(email, mailbox.Principal.OwnerEmail) {
		return nil, nil
	}
	return email, nil
}

func (mailbox McpMailbox) ownedEmails(accountId any) ([]map[string]any, error) {
	domains, err := mailbox.scopedDomains(accountId)
	if err != nil {
		return nil, err
	}
	return mailbox.Emails.List(domains, EmailListFilters{Folder: "all", OwnerEmail: mailbox.Principal.OwnerEmail})
}

func (mailbox McpMailbox) presentMessages(emails []map[string]any) ([]any, error) {
	messages := []any{}
	for _, email := range emails {
		if mailbox.excluded(email) || !(EmailsRecords{}).CanAccessEmail(email, mailbox.Principal.OwnerEmail) {
			continue
		}
		attachments, err := mailbox.Attachments.ListSummaries(TextValue{}.Of(email["id"]))
		if err != nil {
			return nil, err
		}
		messages = append(messages, mailbox.presentMessage(email, attachments))
	}
	return messages, nil
}

func (McpMailbox) presentMessage(email map[string]any, attachments []map[string]any) map[string]any {
	values := RecordValues{}
	to := email["to"]
	if list, _ := values.List(to); len(list) == 0 {
		to = []any{}
		if values.Truthy(email["recipient"]) {
			to = []any{email["recipient"]}
		}
	}
	listed := []any{}
	for _, attachment := range attachments {
		summary := map[string]any{}
		for _, field := range []string{"id", "filename", "contentType", "size"} {
			if value, present := attachment[field]; present {
				summary[field] = value
			}
		}
		listed = append(listed, summary)
	}
	message := map[string]any{
		"id":          email["id"],
		"threadId":    values.Or(email["threadId"], ""),
		"accountId":   email["domain"],
		"folder":      values.Or(email["folder"], "inbox"),
		"from":        values.Or(email["headerFrom"], email["sender"]),
		"to":          to,
		"cc":          values.Or(email["cc"], []any{}),
		"text":        TextValue{}.Prefix(TextValue{}.Of(values.Or(email["body"], "")), 100_000),
		"html":        TextValue{}.Prefix(TextValue{}.Of(values.Or(email["htmlBody"], "")), 100_000),
		"attachments": listed,
	}
	for _, field := range []string{"subject", "read", "starred", "receivedAt"} {
		if value, present := email[field]; present {
			message[field] = value
		}
	}
	return message
}

func (mailbox McpMailbox) threadSummary(thread Thread) map[string]any {
	accounts := []string{}
	for _, email := range thread.Emails {
		if domain := (TextValue{}).Of(email["domain"]); !(PreferenceChoices{}).Includes(accounts, domain) {
			accounts = append(accounts, domain)
		}
	}
	return map[string]any{
		"id":           thread.Id,
		"subject":      thread.Subject,
		"messageCount": thread.Count,
		"participants": thread.Participants,
		"unread":       mailbox.anyEmail(thread, "read", false),
		"starred":      mailbox.anyEmail(thread, "starred", true),
		"latestAt":     thread.LatestEmail["receivedAt"],
		"accountIds":   accounts,
	}
}

// anyEmail reports whether some email of the thread has the field truthy
// (wanted) or falsy (not wanted).
func (McpMailbox) anyEmail(thread Thread, field string, wanted bool) bool {
	for _, email := range thread.Emails {
		if (RecordValues{}).Truthy(email[field]) == wanted {
			return true
		}
	}
	return false
}

func (McpMailbox) trackingView(email map[string]any) map[string]any {
	values := RecordValues{}
	opens, _ := values.List(email["opens"])
	clicks, _ := values.List(email["clicks"])
	last := func(list []any) []any {
		if len(list) > 100 {
			return list[len(list)-100:]
		}
		return append([]any{}, list...)
	}
	return map[string]any{
		"openTracking": email["openTracking"] == true,
		"linkTracking": email["linkTracking"] == true,
		"openCount":    values.Numeric(values.Or(email["openCount"], values.Or(len(opens), 0))),
		"clickCount":   values.Numeric(values.Or(email["clickCount"], values.Or(len(clicks), 0))),
		"opens":        last(opens),
		"clicks":       last(clicks),
	}
}

func (mailbox McpMailbox) scopedDomains(requested any) ([]string, error) {
	if !(RecordValues{}).Truthy(requested) {
		return append([]string{}, mailbox.Principal.AllowedDomains...), nil
	}
	domain := strings.ToLower(Security{}.Trim(TextValue{}.Of(requested)))
	if !(PreferenceChoices{}).Includes(mailbox.Principal.AllowedDomains, domain) {
		return nil, errors.New("Account is not allowed by this MCP capability")
	}
	return []string{domain}, nil
}

func (McpMailbox) requestedFolder(folder any) (string, error) {
	if folder == nil || folder == "" || strings.ToLower(TextValue{}.Of(folder)) == "all" {
		return "all", nil
	}
	path := CategoryPath{}.Normalized(folder)
	if path == "" {
		return "", errors.New("folder must be a safe folder path")
	}
	return path, nil
}

func (mailbox McpMailbox) folderExcluded(domain string, folder string) bool {
	return (PreferenceChoices{}).Includes(mailbox.Principal.ExcludedFolders[domain], strings.ToLower(folder))
}

func (mailbox McpMailbox) excluded(email map[string]any) bool {
	return mailbox.folderExcluded(TextValue{}.Of(email["domain"]), TextValue{}.Of(RecordValues{}.Or(email["folder"], "inbox")))
}

func (mailbox McpMailbox) categories() McpFolders {
	return McpFolders{Users: mailbox.Users, Emails: mailbox.Emails}
}

func (McpMailbox) notFound(err error, message string) error {
	if err != nil {
		return err
	}
	return errors.New(message)
}

func (McpMailbox) number(value any) int {
	number, _ := RecordValues{}.Number(value)
	return int(number)
}

func (McpMailbox) texts(value any) []string {
	list, _ := RecordValues{}.List(value)
	texts := []string{}
	for _, item := range list {
		texts = append(texts, TextValue{}.Of(item))
	}
	return texts
}

func (McpMailbox) first(list []any, limit int) []any {
	if len(list) > limit {
		return list[:limit]
	}
	return list
}
