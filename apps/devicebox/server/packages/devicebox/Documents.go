package devicebox

import "encoding/json"

type Documents struct {
	source Storage
}

func NewDocuments(source Storage) Documents {
	return Documents{source: source}
}

func (documents Documents) Ensure(collection string) error {
	exists, err := documents.Exists(collection)
	if err != nil {
		return err
	}
	return documents.createUnless(exists, collection)
}

func (documents Documents) Exists(collection string) (bool, error) {
	exists := false
	err := documents.into(documents.command("collectionExists", collection), &exists)
	return exists, err
}

func (documents Documents) Insert(collection string, document any) (string, error) {
	command, err := documents.withValue(documents.command("insert", collection), document)
	if err != nil {
		return "", err
	}
	saved := insertAnswer{}
	err = documents.into(command, &saved)
	return saved.Id, err
}

func (documents Documents) Find(collection string, id string, target any) (bool, error) {
	command := documents.command("findById", collection)
	command["id"] = id
	answer, err := documents.call(command)
	if err != nil || answer == nil {
		return false, err
	}
	return true, documents.decode(answer, target)
}

func (documents Documents) Patch(collection string, id string, fields any) error {
	command, err := documents.withValue(documents.command("patch", collection), fields)
	if err != nil {
		return err
	}
	command["id"] = id
	_, err = documents.call(command)
	return err
}

func (documents Documents) Delete(collection string, id string) error {
	command := documents.command("deleteById", collection)
	command["id"] = id
	_, err := documents.call(command)
	return err
}

func (documents Documents) Rows(statement Statement, target any) error {
	answer, err := documents.source.Query(statement.Sql, statement.Parameters)
	if err != nil {
		return Unavailable{}.StoreDown(err)
	}
	return documents.decode(documents.listOrEmpty(answer), target)
}

func (Documents) listOrEmpty(answer any) any {
	if answer == nil {
		return []any{}
	}
	return answer
}

func (documents Documents) createUnless(exists bool, collection string) error {
	if exists {
		return nil
	}
	_, err := documents.call(documents.command("createCollection", collection))
	return err
}

func (documents Documents) withValue(command map[string]any, document any) (map[string]any, error) {
	fields := map[string]any{}
	err := documents.decode(document, &fields)
	command["value"] = fields
	return command, err
}

func (documents Documents) into(command map[string]any, target any) error {
	answer, err := documents.call(command)
	if err != nil {
		return err
	}
	return documents.decode(answer, target)
}

func (documents Documents) call(command map[string]any) (any, error) {
	answer, err := documents.source.Command(command)
	if err != nil {
		return nil, Unavailable{}.StoreDown(err)
	}
	return answer, nil
}

func (Documents) decode(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return Unavailable{}.StoreUnreadable(err)
	}
	err = json.Unmarshal(encoded, target)
	if err != nil {
		return Unavailable{}.StoreUnreadable(err)
	}
	return nil
}

func (Documents) command(name string, collection string) map[string]any {
	return map[string]any{"operation": "document", "command": name, "collection": collection}
}
