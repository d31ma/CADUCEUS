package channels

import "encoding/json"

type CommandDocuments struct {
	source Commander
}

func NewCommandDocuments(source Commander) CommandDocuments {
	return CommandDocuments{source: source}
}

func (documents CommandDocuments) Ensure(collection string) error {
	exists := false
	err := documents.into(documents.command("collectionExists", collection), &exists)
	if err != nil {
		return err
	}
	return documents.createUnless(exists, collection)
}

func (documents CommandDocuments) Insert(collection string, document any) (string, error) {
	command, err := documents.withValue(documents.command("insert", collection), document)
	if err != nil {
		return "", err
	}
	saved := insertAnswer{}
	err = documents.into(command, &saved)
	return saved.Id, err
}

func (documents CommandDocuments) Find(collection string, id string, target any) (bool, error) {
	command := documents.command("findById", collection)
	command["id"] = id
	answer, err := documents.call(command)
	if err != nil || answer == nil {
		return false, err
	}
	return true, documents.decode(answer, target)
}

func (documents CommandDocuments) Replace(collection string, id string, document any) error {
	command, err := documents.withValue(documents.command("patch", collection), document)
	if err != nil {
		return err
	}
	command["id"] = id
	_, err = documents.call(command)
	return err
}

func (documents CommandDocuments) All(collection string, where map[string]any, target any) error {
	rows := []json.RawMessage{}
	window, err := documents.scroll(collection, where, "")
	rows = append(rows, window.Content...)
	for err == nil && window.NextCursor != "" {
		window, err = documents.scroll(collection, where, window.NextCursor)
		rows = append(rows, window.Content...)
	}
	if err != nil {
		return err
	}
	return documents.decode(rows, target)
}

func (documents CommandDocuments) scroll(collection string, where map[string]any, cursor string) (scrollAnswer, error) {
	command := documents.command("scroll", collection)
	command["where"] = where
	command["size"] = documents.pageSize()
	documents.withCursor(command, cursor)
	window := scrollAnswer{}
	return window, documents.into(command, &window)
}

func (CommandDocuments) withCursor(command map[string]any, cursor string) {
	if cursor != "" {
		command["cursor"] = cursor
	}
}

func (documents CommandDocuments) createUnless(exists bool, collection string) error {
	if exists {
		return nil
	}
	_, err := documents.call(documents.command("createCollection", collection))
	return err
}

func (documents CommandDocuments) withValue(command map[string]any, document any) (map[string]any, error) {
	fields := map[string]any{}
	err := documents.decode(document, &fields)
	command["value"] = fields
	return command, err
}

func (documents CommandDocuments) into(command map[string]any, target any) error {
	answer, err := documents.call(command)
	if err != nil {
		return err
	}
	return documents.decode(answer, target)
}

func (documents CommandDocuments) call(command map[string]any) (any, error) {
	answer, err := documents.source.Command(command)
	if err != nil {
		return nil, &Unavailable{Message: "the channel store is not available", Cause: err}
	}
	return answer, nil
}

func (CommandDocuments) decode(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return &Unavailable{Message: "the channel store answered something unreadable", Cause: err}
	}
	err = json.Unmarshal(encoded, target)
	if err != nil {
		return &Unavailable{Message: "the channel store answered something unreadable", Cause: err}
	}
	return nil
}

func (CommandDocuments) command(name string, collection string) map[string]any {
	return map[string]any{"operation": "document", "command": name, "collection": collection}
}

func (CommandDocuments) pageSize() int {
	return 100
}
