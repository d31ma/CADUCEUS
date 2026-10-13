// McpToolCatalog declares the 21 mailbox tools of the MCP endpoint and
// writes their JSON Schemas as the JavaScript MCP SDK did from zod.
type McpToolCatalog struct{}

// Tools answers every tool in listing order.
func (catalog McpToolCatalog) Tools() []McpTool {
	account := catalog.text("accountId", -1, 253, true)
	folder := catalog.text("folder", -1, 60, true)
	threadId := catalog.text("threadId", 1, 500, false)
	messageId := catalog.text("messageId", 1, 500, false)
	threadIds := catalog.list("threadIds", catalog.text("", 1, 500, false), 1, 100)
	draftId := catalog.text("draftId", 1, 500, false)
	return []McpTool{
		{"list_accounts", "read", "List mail accounts allowed by this MCP capability.", nil},
		{"list_folders", "read", "List allowed folders and stable system roles.", []McpToolField{account}},
		{"search_mail", "read", "Search exact-owner mail across allowed accounts and folders.", []McpToolField{
			catalog.defaulted(catalog.text("query", -1, 1_000, false), ""), account, folder, catalog.number("limit", 1, 100, 50),
		}},
		{"list_threads", "read", "List exact-owner conversations in an allowed scope.", []McpToolField{
			account, folder, catalog.flag("unread"), catalog.flag("starred"),
			catalog.number("offset", 0, 100_000, 0), catalog.number("limit", 1, 100, 50),
		}},
		{"get_thread", "read", "Read one allowed conversation and its messages.", []McpToolField{threadId}},
		{"get_message", "read", "Read one exact-owner message.", []McpToolField{messageId}},
		{"get_message_tracking", "read", "Return local open and click activity for an owned sent message.", []McpToolField{messageId}},
		{"list_tracked_messages", "read", "List owned messages with local open or link tracking.", []McpToolField{account, catalog.number("limit", 1, 100, 50)}},
		{"get_top_links", "read", "Aggregate clicked links from locally tracked sent messages.", []McpToolField{account, catalog.number("limit", 1, 50, 20)}},
		{"list_scheduled_sends", "read", "List this mailbox’s pending scheduled sends.", nil},
		{"create_draft", "write", "Create a durable draft without sending it.", catalog.draftFields()},
		{"reply_to_thread", "write", "Create a durable reply draft for an allowed conversation.", []McpToolField{
			threadId, catalog.defaulted(McpToolField{Name: "replyAll", Kind: "boolean"}, false),
			catalog.defaulted(catalog.text("text", -1, 5_000_000, false), ""), catalog.defaulted(catalog.text("html", -1, 5_000_000, false), ""),
		}},
		{"archive_threads", "write", "Move complete allowed conversations to each account’s Archive folder.", []McpToolField{threadIds}},
		{"trash_threads", "write", "Move complete allowed conversations to each account’s Trash folder.", []McpToolField{threadIds}},
		{"move_to_folder", "write", "Move complete allowed conversations to one physical folder.", []McpToolField{
			threadIds, catalog.text("accountId", 1, 253, false), catalog.text("folder", 1, 60, false),
		}},
		{"set_labels", "write", "Set provider labels when the selected account supports labels.", []McpToolField{
			threadIds, catalog.list("labels", catalog.text("", 1, 100, false), -1, 100),
		}},
		{"set_unread", "write", "Mark complete allowed conversations read or unread.", []McpToolField{threadIds, {Name: "unread", Kind: "boolean"}}},
		{"set_starred", "write", "Star or unstar complete allowed conversations.", []McpToolField{threadIds, {Name: "starred", Kind: "boolean"}}},
		{"cancel_scheduled_send", "write", "Cancel one exact-owner scheduled send.", []McpToolField{catalog.text("id", 1, 500, false)}},
		{"send_draft", "send", "Send one exact-owner draft immediately.", []McpToolField{draftId}},
		{"schedule_send", "send", "Schedule one exact-owner draft for future delivery.", []McpToolField{draftId, {Name: "sendAt", Kind: "string", Format: "date-time"}}},
	}
}

// Find answers the tool with a name.
func (catalog McpToolCatalog) Find(name string) (McpTool, bool) {
	for _, tool := range catalog.Tools() {
		if tool.Name == name {
			return tool, true
		}
	}
	return McpTool{}, false
}

// Listing answers the tools/list result.
func (catalog McpToolCatalog) Listing() McpJsonObject {
	tools := []any{}
	for _, tool := range catalog.Tools() {
		annotations := McpJsonObject{}.
			With("readOnlyHint", tool.Category == "read").
			With("destructiveHint", tool.Name == "trash_threads" || tool.Name == "cancel_scheduled_send").
			With("idempotentHint", !(tool.Name == "create_draft" || tool.Name == "reply_to_thread" || tool.Name == "send_draft" || tool.Name == "schedule_send"))
		tools = append(tools, McpJsonObject{}.
			With("name", tool.Name).
			With("description", tool.Description).
			With("inputSchema", catalog.schema(tool)).
			With("annotations", annotations))
	}
	return McpJsonObject{}.With("tools", tools)
}

func (catalog McpToolCatalog) schema(tool McpTool) McpJsonObject {
	properties := McpJsonObject{}
	required := []string{}
	for _, field := range tool.Fields {
		properties = properties.With(field.Name, catalog.property(field))
		if field.Default == nil && !field.Optional {
			required = append(required, field.Name)
		}
	}
	schema := McpJsonObject{}.With("type", "object").With("$schema", "https://json-schema.org/draft/2020-12/schema").With("properties", properties)
	if len(required) > 0 {
		schema = schema.With("required", required)
	}
	return schema
}

func (catalog McpToolCatalog) property(field McpToolField) McpJsonObject {
	property := McpJsonObject{}
	if field.Default != nil {
		property = property.With("default", field.Default)
	}
	if field.Kind == "array" {
		if field.Minimum != nil {
			property = property.With("minItems", *field.Minimum)
		}
		if field.Maximum != nil {
			property = property.With("maxItems", *field.Maximum)
		}
		return property.With("type", "array").With("items", catalog.property(*field.Items))
	}
	property = property.With("type", field.Kind)
	switch field.Format {
	case "email":
		property = property.With("format", "email").With("pattern", McpArguments{}.EmailPattern())
	case "date-time":
		property = property.With("format", "date-time").With("pattern", McpArguments{}.DateTimePattern())
	}
	names := map[string][2]string{"string": {"minLength", "maxLength"}, "integer": {"minimum", "maximum"}}[field.Kind]
	if field.Minimum != nil {
		property = property.With(names[0], *field.Minimum)
	}
	if field.Maximum != nil {
		property = property.With(names[1], *field.Maximum)
	}
	return property
}

func (catalog McpToolCatalog) draftFields() []McpToolField {
	email := McpToolField{Kind: "string", Format: "email"}
	addresses := func(name string) McpToolField {
		return catalog.defaulted(catalog.list(name, email, -1, 100), []any{})
	}
	from := email
	from.Name, from.Optional = "from", true
	return []McpToolField{
		from, addresses("to"), addresses("cc"), addresses("bcc"),
		catalog.defaulted(catalog.text("subject", -1, 500, false), ""),
		catalog.defaulted(catalog.text("text", -1, 5_000_000, false), ""),
		catalog.defaulted(catalog.text("html", -1, 5_000_000, false), ""),
	}
}

// text declares a string; a negative minimum leaves it unbounded below.
func (catalog McpToolCatalog) text(name string, minimum int, maximum int, optional bool) McpToolField {
	return McpToolField{Name: name, Kind: "string", Minimum: catalog.bound(minimum), Maximum: &maximum, Optional: optional}
}

func (McpToolCatalog) number(name string, minimum int, maximum int, fallback int) McpToolField {
	return McpToolField{Name: name, Kind: "integer", Minimum: &minimum, Maximum: &maximum, Default: fallback}
}

func (McpToolCatalog) flag(name string) McpToolField {
	return McpToolField{Name: name, Kind: "boolean", Optional: true}
}

func (catalog McpToolCatalog) list(name string, item McpToolField, minimum int, maximum int) McpToolField {
	return McpToolField{Name: name, Kind: "array", Items: &item, Minimum: catalog.bound(minimum), Maximum: &maximum}
}

func (McpToolCatalog) defaulted(field McpToolField, fallback any) McpToolField {
	field.Default = fallback
	return field
}

func (McpToolCatalog) bound(value int) *int {
	if value < 0 {
		return nil
	}
	return &value
}
