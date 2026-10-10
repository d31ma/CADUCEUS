import (
	"encoding/json"
)

// ConversationArchive archives a whole conversation with an undo-safe folder
// snapshot, for Send & Archive. Mail reading owns the full archive service;
// this is the minimal private version delivery needs.
type ConversationArchive struct {
	Emails  *EmailsHelper
	Threads *ThreadsHelper
	Users   *UsersHelper
}

// Plan resolves and authorizes the source conversation before a send is
// accepted: {sourceEmailId, threadId, messages: [{id, domain, folder}]}.
func (archive ConversationArchive) Plan(sourceEmailId string, ownerEmail string, domains []string) (map[string]any, error) {
	_, selected, err := archive.Emails.FindById(sourceEmailId)
	if err != nil {
		return nil, err
	}
	notFound := &DeliveryRefusal{Message: "Source conversation not found", Status: 404}
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
	roles := CategoryRoles{Users: archive.Users}
	messages := []any{}
	for _, email := range thread.Emails {
		domain := RecordValues{}.Text(email["domain"])
		folder := RecordValues{}.Text(RecordValues{}.Or(email["folder"], "inbox"))
		terminal := false
		for _, role := range []string{"archive", "drafts", "sent", "trash"} {
			terminal = terminal || roles.PathFor(ownerEmail, domain, role) == folder
		}
		if !terminal {
			messages = append(messages, map[string]any{"id": email["id"], "domain": domain, "folder": folder})
		}
	}
	if len(messages) == 0 {
		return nil, &DeliveryRefusal{Message: "Source conversation is already archived", Status: 409}
	}
	return map[string]any{"sourceEmailId": sourceEmailId, "threadId": thread.Id, "messages": messages}, nil
}

// Archive moves the exact pre-authorized snapshot to the archive folder.
func (archive ConversationArchive) Archive(plan map[string]any, ownerEmail string) (map[string]any, error) {
	moved := 0
	messages, _ := RecordValues{}.List(plan["messages"])
	for _, item := range messages {
		message, _ := RecordValues{}.Object(item)
		documentId, current, err := archive.Emails.FindById(RecordValues{}.Text(message["id"]))
		if err != nil {
			return nil, err
		}
		if documentId == "" || current == nil || (RecordValues{}).Text(current["folder"]) != message["folder"] {
			continue
		}
		folder := CategoryRoles{Users: archive.Users}.PathFor(ownerEmail, RecordValues{}.Text(message["domain"]), "archive")
		if err := archive.Emails.Update(documentId, map[string]any{"folder": folder}); err != nil {
			return nil, err
		}
		moved++
	}
	if moved != len(messages) {
		return nil, &DeliveryRefusal{Message: "Source conversation changed before it could be archived", Status: 409}
	}
	return map[string]any{"archived": true, "archivedCount": moved, "archiveThreadId": plan["threadId"]}, nil
}

// Serialized answers the plan as the JSON undo state.
func (ConversationArchive) Serialized(plan map[string]any) string {
	return StoredRecords{}.JsonText(map[string]any{"sourceEmailId": plan["sourceEmailId"], "threadId": plan["threadId"], "messages": plan["messages"]})
}

// Restore moves back only the messages still archived, keeping later moves,
// and answers how many it restored.
func (archive ConversationArchive) Restore(serialized string, ownerEmail string, domains []string) (int, error) {
	state := map[string]any{}
	if json.Unmarshal([]byte(serialized), &state) != nil {
		return 0, nil
	}
	messages, isList := RecordValues{}.List(state["messages"])
	if !isList || len(messages) > 500 {
		return 0, nil
	}
	order, byFolder := []string{}, map[string][]string{}
	for _, item := range messages {
		message, _ := RecordValues{}.Object(item)
		id, idText := message["id"].(string)
		folder, folderText := message["folder"].(string)
		if !idText || !folderText {
			continue
		}
		_, email, err := archive.Emails.FindById(id)
		if err != nil {
			return 0, err
		}
		if email == nil {
			continue
		}
		archived := CategoryRoles{Users: archive.Users}.PathFor(ownerEmail, RecordValues{}.Text(email["domain"]), "archive")
		if email["folder"] != archived || !(TextValue{}).Includes(domains, email["domain"]) || !(EmailsRecords{}).CanAccessEmail(email, ownerEmail) {
			continue
		}
		if _, seen := byFolder[folder]; !seen {
			order = append(order, folder)
		}
		byFolder[folder] = append(byFolder[folder], RecordValues{}.Text(email["id"]))
	}
	restored := 0
	for _, folder := range order {
		updated, err := archive.Emails.UpdateByIds(byFolder[folder], map[string]any{"folder": folder})
		if err != nil {
			return restored, err
		}
		restored += len(updated)
	}
	return restored, nil
}
