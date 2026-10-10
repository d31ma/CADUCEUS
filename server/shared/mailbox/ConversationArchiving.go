import "encoding/json"

// ConversationArchiving archives a whole conversation when a reply is sent,
// from a bounded snapshot taken and authorized before the send, and
// restores it on undo without overriding later moves.
type ConversationArchiving struct {
	Emails     *EmailsHelper
	Threads    *ThreadsHelper
	Categories *CategoryMappingService
}

// Plan authorizes the source email and answers the snapshot of the
// conversation's messages outside the archive, drafts, sent and trash roles.
func (archive ConversationArchiving) Plan(sourceEmailId string, ownerEmail string, domains []string) (map[string]any, error) {
	notFound := &ConversationArchiveError{Message: "Source conversation not found", Status: 404}
	_, selected, err := archive.Emails.FindById(sourceEmailId)
	if err != nil {
		return nil, err
	}
	if selected == nil || !(TextValue{}).Includes(domains, selected["domain"]) || !(EmailsRecords{}).CanAccessEmail(selected, ownerEmail) {
		return nil, notFound
	}
	thread, err := archive.Threads.FindForEmail(domains, sourceEmailId, ownerEmail)
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, notFound
	}
	mappings, err := archive.Categories.ForOwner(ownerEmail)
	if err != nil {
		return nil, err
	}
	messages := []any{}
	for _, email := range thread.Emails {
		domain := TextValue{}.Of(email["domain"])
		folder := TextValue{}.Of(TextValue{}.Or(email["folder"], "inbox"))
		terminal := false
		for _, role := range []string{"archive", "drafts", "sent", "trash"} {
			terminal = terminal || ((TextValue{}).Includes(domains, domain) && mappings.PathFor(domain, role) == folder)
		}
		if !terminal {
			messages = append(messages, map[string]any{"id": email["id"], "domain": email["domain"], "folder": folder})
		}
	}
	if len(messages) == 0 {
		return nil, &ConversationArchiveError{Message: "Source conversation is already archived", Status: 409}
	}
	return map[string]any{"sourceEmailId": sourceEmailId, "threadId": thread.Id, "messages": messages}, nil
}

// Archive moves the planned messages that are still where the plan saw
// them, and refuses when any of them moved since.
func (archive ConversationArchiving) Archive(plan map[string]any, ownerEmail string) (map[string]any, error) {
	mappings, err := archive.Categories.ForOwner(ownerEmail)
	if err != nil {
		return nil, err
	}
	planned := TextValue{}.List(plan["messages"])
	moved := 0
	for _, item := range planned {
		message, _ := item.(map[string]any)
		documentId, current, err := archive.Emails.FindById(TextValue{}.Of(message["id"]))
		if err != nil {
			return nil, err
		}
		if documentId == "" || current == nil || current["folder"] != message["folder"] {
			continue
		}
		folder := TextValue{}.Or(mappings.PathFor(TextValue{}.Of(message["domain"]), "archive"), "archive")
		if err := archive.Emails.Update(documentId, map[string]any{"folder": folder}); err != nil {
			return nil, err
		}
		moved++
	}
	if moved != len(planned) {
		return nil, &ConversationArchiveError{Message: "Source conversation changed before it could be archived", Status: 409}
	}
	return map[string]any{"archived": true, "archivedCount": moved, "archiveThreadId": plan["threadId"]}, nil
}

// Serialized answers the plan as the JSON text a send's undo keeps.
func (ConversationArchiving) Serialized(plan map[string]any) string {
	return TextValue{}.Json(map[string]any{"sourceEmailId": plan["sourceEmailId"], "threadId": plan["threadId"], "messages": plan["messages"]})
}

// Restore moves the messages a serialized plan archived back to their
// folders, when they are still archived, and answers how many moved.
func (archive ConversationArchiving) Restore(serialized string, ownerEmail string, domains []string) (int, error) {
	var state map[string]any
	if serialized == "" || json.Unmarshal([]byte(serialized), &state) != nil || state == nil {
		return 0, nil
	}
	items, isList := state["messages"].([]any)
	if !isList || len(items) > 500 {
		return 0, nil
	}
	mappings, err := archive.Categories.ForOwner(ownerEmail)
	if err != nil {
		return 0, err
	}
	folders := []string{}
	byFolder := map[string][]string{}
	for _, item := range items {
		fields, _ := item.(map[string]any)
		id, hasId := fields["id"].(string)
		folder, hasFolder := fields["folder"].(string)
		if !hasId || !hasFolder {
			continue
		}
		_, email, err := archive.Emails.FindById(id)
		if err != nil {
			return 0, err
		}
		if !archive.restorable(email, mappings, ownerEmail, domains) {
			continue
		}
		if _, seen := byFolder[folder]; !seen {
			folders = append(folders, folder)
		}
		byFolder[folder] = append(byFolder[folder], TextValue{}.Of(email["id"]))
	}
	return archive.restored(folders, byFolder)
}

// restorable reports whether an email is still archived and the owner's.
func (ConversationArchiving) restorable(email map[string]any, mappings CategoryMappings, ownerEmail string, domains []string) bool {
	if email == nil {
		return false
	}
	archived := TextValue{}.Or(mappings.PathFor(TextValue{}.Of(email["domain"]), "archive"), "archive")
	return email["folder"] == archived && (TextValue{}).Includes(domains, email["domain"]) && (EmailsRecords{}).CanAccessEmail(email, ownerEmail)
}

func (archive ConversationArchiving) restored(folders []string, byFolder map[string][]string) (int, error) {
	restored := 0
	for _, folder := range folders {
		matched, err := archive.Emails.UpdateByIds(byFolder[folder], map[string]any{"folder": folder})
		if err != nil {
			return restored, err
		}
		restored += len(matched)
	}
	return restored, nil
}
