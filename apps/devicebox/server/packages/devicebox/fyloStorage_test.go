package devicebox

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type fyloStorage struct {
	input  io.WriteCloser
	output *bufio.Reader
}

func fyloBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("MESAIL_FYLO")
	if binary == "" {
		home, _ := os.UserHomeDir()
		binary = filepath.Join(home, "Developer", "Tools", "fylo-26.45.07", "fylo")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("no fylo binary at %s (set MESAIL_FYLO)", binary)
	}
	return binary
}

func newFyloStorage(t *testing.T) *fyloStorage {
	t.Helper()
	command := exec.Command(fyloBinary(t), "exec", "--loop", "--root", t.TempDir())
	command.Env = append(os.Environ(), "FYLO_ALLOW_SYSTEM_DISK=1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		command.Wait()
	})
	return &fyloStorage{input: input, output: bufio.NewReaderSize(output, 1<<20)}
}

func (storage *fyloStorage) request(fields map[string]any) (any, error) {
	encoded, _ := json.Marshal(fields)
	if _, err := storage.input.Write(append(encoded, '\n')); err != nil {
		return nil, err
	}
	line, err := storage.output.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	answer := struct {
		Ok     bool `json:"ok"`
		Result any  `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}{}
	if err := json.Unmarshal(line, &answer); err != nil {
		return nil, err
	}
	if !answer.Ok {
		return nil, errors.New(answer.Error.Message)
	}
	return answer.Result, nil
}

func (storage *fyloStorage) Command(command map[string]any) (any, error) {
	collection := command["collection"]
	switch command["command"] {
	case "collectionExists":
		result, err := storage.request(map[string]any{"op": "inspectCollection", "collection": collection})
		if err != nil {
			return nil, err
		}
		return result.(map[string]any)["exists"] == true, nil
	case "createCollection":
		_, err := storage.request(map[string]any{"op": "createCollection", "collection": collection})
		return true, err
	case "insert":
		id, err := storage.request(map[string]any{"op": "putData", "collection": collection, "data": command["value"]})
		saved := map[string]any{}
		for key, value := range command["value"].(map[string]any) {
			saved[key] = value
		}
		saved["id"] = id
		return saved, err
	case "findById":
		return storage.latest(collection, command["id"])
	case "patch":
		if _, err := storage.request(map[string]any{"op": "patchDoc", "collection": collection, "id": command["id"], "newDoc": command["value"]}); err != nil {
			return nil, err
		}
		return storage.latest(collection, command["id"])
	case "deleteById":
		_, err := storage.request(map[string]any{"op": "delDoc", "collection": collection, "id": command["id"]})
		if err != nil && strings.Contains(err.Error(), "NOT_FOUND") {
			return false, nil
		}
		return err == nil, err
	}
	return nil, fmt.Errorf("unsupported command %v", command["command"])
}

func (storage *fyloStorage) latest(collection any, id any) (any, error) {
	result, err := storage.request(map[string]any{"op": "getLatest", "collection": collection, "id": id})
	if err != nil {
		return nil, err
	}
	rows := storage.rows(result)
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (storage *fyloStorage) Query(sql string, parameters []any) (any, error) {
	for index := len(parameters); index >= 1; index-- {
		sql = strings.ReplaceAll(sql, fmt.Sprintf("$%d", index), storage.literal(parameters[index-1]))
	}
	result, err := storage.request(map[string]any{"op": "executeSQL", "sql": sql})
	if err != nil {
		return nil, err
	}
	return storage.rows(result), nil
}

func (storage *fyloStorage) explain(sql string, parameters []any) map[string]any {
	for index := len(parameters); index >= 1; index-- {
		sql = strings.ReplaceAll(sql, fmt.Sprintf("$%d", index), storage.literal(parameters[index-1]))
	}
	result, _ := storage.request(map[string]any{"op": "executeSQL", "sql": "EXPLAIN " + sql})
	plan, _ := result.(map[string]any)
	return plan
}

func (storage *fyloStorage) literal(value any) string {
	switch typed := value.(type) {
	case string:
		return "'" + strings.ReplaceAll(typed, "'", "''") + "'"
	default:
		return fmt.Sprint(typed)
	}
}

func (storage *fyloStorage) rows(result any) []any {
	byId, _ := result.(map[string]any)
	ids := []string{}
	for id := range byId {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := []any{}
	for _, id := range ids {
		row, _ := byId[id].(map[string]any)
		copied := map[string]any{"id": id}
		for key, value := range row {
			copied[key] = value
		}
		rows = append(rows, copied)
	}
	return rows
}
